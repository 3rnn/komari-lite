package metric

import (
	"bytes"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"
	"testing"
	"time"
)

func TestSQLiteV4BudgetedDiscoveryFiltersOnlyAfterIndexedCandidateLimit(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := store.CreateMetric(ctx, Definition{Name: "budget.tag", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, Point{MetricName: "budget.tag", EntityID: "n1", Timestamp: stamp, Value: 42, Tags: map[string]string{"group": "match"}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 64; i++ {
		if _, err := store.db.ExecContext(ctx, `INSERT INTO metric_series (metric_name, entity_id, tags_hash, tags) VALUES (?, ?, ?, ?)`, "budget.tag", "n1", fmt.Sprintf("fake-%03d", i), `{"group":"other"}`); err != nil {
			t.Fatal(err)
		}
	}
	q := Query{MetricName: "budget.tag", EntityID: "n1", Tags: map[string]string{"group": "absent"}, Start: stamp, End: stamp, WorkBudget: 3}
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("missing tag must bound candidate discovery: %v %v", got, err)
	}
	q.WorkBudget = 65
	if got, err := store.Query(ctx, q); err != nil || len(got) != 0 {
		t.Fatalf("missing tag within bound: %v %v", got, err)
	}
	q.Tags["group"] = "match"
	if got, err := store.Query(ctx, q); err != nil || len(got) != 1 || got[0].Value != 42 {
		t.Fatalf("matching tag within bound: %v %v", got, err)
	}
}

func TestSQLiteV4BudgetedDiscoveryUsesSeriesIndexWithoutFullSort(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rows, err := store.db.QueryContext(ctx, `EXPLAIN QUERY PLAN SELECT id, metric_name, entity_id, tags_hash, tags FROM metric_series WHERE metric_name = ? AND entity_id = ? ORDER BY metric_name, entity_id, tags_hash LIMIT ?`, "cpu", "n1", 4)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var details string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		details += detail
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(details, "INDEX") || strings.Contains(details, "TEMP B-TREE") {
		t.Fatalf("discovery must use index without sorting all candidates: %s", details)
	}
}

func TestSQLiteV4BudgetedBlockPayloadRejectsBeforeLoadAndInflation(t *testing.T) {
	const maxTestPayload = 4 << 20
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := store.CreateMetric(ctx, Definition{Name: "budget.bytes", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, Point{MetricName: "budget.bytes", EntityID: "n1", Timestamp: stamp, Value: 5}); err != nil {
		t.Fatal(err)
	}
	var seriesID int64
	if err := store.db.QueryRowContext(ctx, `SELECT id FROM metric_series WHERE metric_name = 'budget.bytes'`).Scan(&seriesID); err != nil {
		t.Fatal(err)
	}
	q := Query{MetricName: "budget.bytes", EntityID: "n1", Start: stamp, End: stamp, WorkBudget: 3}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO metric_point_blocks (series_id, start_nano, end_nano, point_count, codec, checksum, payload) VALUES (?, ?, ?, 1, 1, 0, zeroblob(?))`, seriesID, stamp.UnixNano(), stamp.UnixNano(), maxTestPayload+1); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("large payload should reject before read: %v %v", got, err)
	}
	var packed bytes.Buffer
	packed.WriteByte(sqliteV4PayloadDeflate)
	w, err := flate.NewWriter(&packed, flate.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(bytes.Repeat([]byte{'x'}, maxTestPayload+1)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE metric_point_blocks SET payload = ?, checksum = ? WHERE series_id = ?`, packed.Bytes(), int64(crc32.ChecksumIEEE(packed.Bytes())), seriesID); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("compressed expansion should reject: %v %v", got, err)
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM metric_point_blocks WHERE series_id = ?`, seriesID); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Query(ctx, q); err != nil || len(got) != 1 || got[0].Value != 5 {
		t.Fatalf("ordinary bounded hot point changed: %v %v", got, err)
	}
}
