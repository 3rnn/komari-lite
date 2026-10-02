package metric

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSQLiteV4BudgetedTagsRejectOversizeBeforeDecode(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := store.db.ExecContext(ctx, `INSERT INTO metric_series (metric_name, entity_id, tags_hash, tags) VALUES ('large.tags', 'n1', 'giant', ?)`, strings.Repeat("x", sqliteV4BudgetedPayloadBytes+1)); err != nil {
		t.Fatal(err)
	}
	q := Query{MetricName: "large.tags", EntityID: "n1", Start: stamp, End: stamp, WorkBudget: 2000}
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("giant tags should be rejected before decoding: %v %v", got, err)
	}
}

func TestSQLiteV4BudgetedTagsRejectCumulativeBytes(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	tag := `{"v":"` + strings.Repeat("x", sqliteV4BudgetedPayloadBytes/3) + `"}`
	for i := 0; i < 4; i++ {
		if _, err := store.db.ExecContext(ctx, `INSERT INTO metric_series (metric_name, entity_id, tags_hash, tags) VALUES ('many.tags', 'n1', ?, ?)`, fmt.Sprintf("%03d", i), tag); err != nil {
			t.Fatal(err)
		}
	}
	q := Query{MetricName: "many.tags", EntityID: "n1", Start: stamp, End: stamp, WorkBudget: 2000}
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("combined tags should reject entire query: %v %v", got, err)
	}
	// Ordinary internal discovery retains its original unrestricted semantics.
	series, err := store.sqliteV4MatchingSeries(ctx, store.db, "many.tags", "n1", nil)
	if err != nil || len(series) != 4 {
		t.Fatalf("internal discovery changed: %d %v", len(series), err)
	}
}

func TestSQLiteV4BudgetedHotLabelsRejectGiantAndCumulativeBytes(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := store.CreateMetric(ctx, Definition{Name: "large.labels", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, Point{MetricName: "large.labels", EntityID: "n1", Timestamp: stamp, Value: 7}); err != nil {
		t.Fatal(err)
	}
	var seriesID int64
	if err := store.db.QueryRowContext(ctx, `SELECT id FROM metric_series WHERE metric_name='large.labels'`).Scan(&seriesID); err != nil {
		t.Fatal(err)
	}
	q := Query{MetricName: "large.labels", EntityID: "n1", Start: stamp, End: stamp.Add(3 * time.Second), WorkBudget: 2000}
	if _, err := store.db.ExecContext(ctx, `UPDATE metric_point_values SET labels=? WHERE series_id=?`, strings.Repeat("x", sqliteV4BudgetedPayloadBytes+1), seriesID); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("giant labels should reject before loading: %v %v", got, err)
	}
	labels := `{"v":"` + strings.Repeat("x", sqliteV4BudgetedPayloadBytes/3) + `"}`
	if _, err := store.db.ExecContext(ctx, `UPDATE metric_point_values SET labels=? WHERE series_id=?`, labels, seriesID); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 4; i++ {
		if _, err := store.db.ExecContext(ctx, `INSERT INTO metric_point_values (series_id,ts_nano,value,labels,created_at) VALUES (?,?,?,?,?)`, seriesID, stamp.Add(time.Duration(i)*time.Second).UnixNano(), float64(i), labels, stamp.UnixNano()); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("combined labels should reject entire query: %v %v", got, err)
	}
	q.WorkBudget = 0
	if got, err := store.Query(ctx, q); err != nil || len(got) != 4 || got[0].Value != 7 {
		t.Fatalf("internal query changed: %d %v", len(got), err)
	}
}

func TestSQLiteV4BudgetedMetadataPreflightCancellation(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := store.CreateMetric(ctx, Definition{Name: "cancel.metadata", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, Point{MetricName: "cancel.metadata", EntityID: "n1", Timestamp: stamp, Value: 1}); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	q := Query{MetricName: "cancel.metadata", EntityID: "n1", Start: stamp, End: stamp, WorkBudget: 2000}
	if got, err := store.Query(canceled, q); !errors.Is(err, context.Canceled) || len(got) != 0 {
		t.Fatalf("canceled preflight should fail: %v %v", got, err)
	}
}

func TestSQLiteV4BudgetedMetadataCountsUTF8BytesNotCharacters(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	large := `{"v":"` + strings.Repeat("界", sqliteV4BudgetedPayloadBytes/3) + `"}`
	if _, err := store.db.ExecContext(ctx, `INSERT INTO metric_series (metric_name, entity_id, tags_hash, tags) VALUES ('utf8.tags', 'n1', 'utf8', ?)`, large); err != nil {
		t.Fatal(err)
	}
	q := Query{MetricName: "utf8.tags", EntityID: "n1", Start: stamp, End: stamp, WorkBudget: 2000}
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("UTF-8 tag byte budget: %v %v", got, err)
	}
	if err := store.CreateMetric(ctx, Definition{Name: "utf8.labels", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, Point{MetricName: "utf8.labels", EntityID: "n1", Timestamp: stamp, Value: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE metric_point_values SET labels=? WHERE series_id=(SELECT id FROM metric_series WHERE metric_name='utf8.labels')`, large); err != nil {
		t.Fatal(err)
	}
	q.MetricName = "utf8.labels"
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("UTF-8 label byte budget: %v %v", got, err)
	}
}
