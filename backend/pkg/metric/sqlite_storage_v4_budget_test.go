package metric

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteV4RawQueryWorkBudgetBoundsSeriesDiscovery(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.CreateMetric(ctx, Definition{Name: "budget.series", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := store.Write(ctx, Point{MetricName: "budget.series", EntityID: "n1", Timestamp: stamp, Value: 7, Tags: map[string]string{"group": "000"}}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 64; i++ {
		if _, err := store.db.ExecContext(ctx,
			`INSERT INTO metric_series (metric_name, entity_id, tags_hash, tags) VALUES (?, ?, ?, ?)`,
			"budget.series", "n1", fmt.Sprintf("series-%03d", i), fmt.Sprintf(`{"group":"%03d"}`, i)); err != nil {
			t.Fatal(err)
		}
	}
	query := Query{MetricName: "budget.series", EntityID: "n1", Start: stamp, End: stamp, WorkBudget: 3, Limit: 1}
	if got, err := store.Query(ctx, query); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("expected full rejection for 64 series but one point: points=%v err=%v", got, err)
	}
	query.WorkBudget = 0
	if got, err := store.Query(ctx, query); err != nil || len(got) != 1 || got[0].Value != 7 {
		t.Fatalf("unrestricted query changed: points=%v err=%v", got, err)
	}
	// A row beyond budget+1 must never be decoded by the budgeted query.
	if _, err := store.db.ExecContext(ctx,
		`UPDATE metric_series SET tags = 'not-json' WHERE metric_name = ? AND tags_hash = ?`,
		"budget.series", "series-063"); err != nil {
		t.Fatal(err)
	}
	query.WorkBudget = 3
	if got, err := store.Query(ctx, query); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("expected budget rejection before decoding distant series: points=%v err=%v", got, err)
	}
}

func TestSQLiteV4RawQueryWorkBudgetCapsSeriesSQLVariables(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.ExecContext(ctx, `WITH RECURSIVE n(i) AS (
		SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 996
	) INSERT INTO metric_series (metric_name, entity_id, tags_hash, tags)
	SELECT 'budget.cap', 'n1', printf('series-%04d', i), '{}' FROM n`); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	query := Query{MetricName: "budget.cap", EntityID: "n1", Start: stamp, End: stamp, WorkBudget: 2000}
	if got, err := store.Query(ctx, query); err != nil || len(got) != 0 {
		t.Fatalf("query at safe series limit changed: points=%v err=%v", got, err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO metric_series (metric_name, entity_id, tags_hash, tags)
		VALUES ('budget.cap', 'n1', 'series-0997', '{}')`); err != nil {
		t.Fatal(err)
	}
	// Internal discovery is still unlimited; only the budgeted raw query is capped.
	series, err := store.sqliteV4MatchingSeries(ctx, store.db, "budget.cap", "n1", nil)
	if err != nil || len(series) != 997 {
		t.Fatalf("internal series discovery changed: count=%d err=%v", len(series), err)
	}
	if got, err := store.Query(ctx, query); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("expected rejection before constructing an oversized IN clause: points=%v err=%v", got, err)
	}
}

func TestSQLiteV4RawQueryWorkBudgetRejectsBeforeDecodingBlocks(t *testing.T) {
	ctx := context.Background()
	dsn := sqliteFileDSN(filepath.Join(t.TempDir(), "metrics.db"))
	store := createSQLiteV3OnlyStore(t, ctx, dsn)
	if err := store.CreateMetric(ctx, Definition{Name: "budget.blocks", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	points := make([]Point, 2101)
	for i := range points {
		points[i] = Point{MetricName: "budget.blocks", EntityID: "n1", Timestamp: base.Add(time.Duration(i) * time.Second), Value: float64(i)}
	}
	if err := store.WriteBatch(ctx, points); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, SQLite(dsn))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var blocks int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM metric_point_blocks`).Scan(&blocks); err != nil {
		t.Fatal(err)
	}
	if blocks < 2 {
		t.Fatalf("expected multiple V4 blocks, got %d", blocks)
	}
	// Invalid payload would cause a decode error if the over-budget query reached it.
	if _, err := store.db.ExecContext(ctx, `UPDATE metric_point_blocks SET payload = x'00'`); err != nil {
		t.Fatal(err)
	}
	query := Query{MetricName: "budget.blocks", EntityID: "n1", Start: base, End: points[len(points)-1].Timestamp, Limit: 2001, WorkBudget: 2000}
	if _, err := store.Query(ctx, query); !errors.Is(err, ErrQueryWorkBudgetExceeded) {
		t.Fatalf("expected pre-decode budget rejection, got %v", err)
	}
}

func TestSQLiteV4RawQueryWorkBudgetRejectsHotPointsWithoutChangingInternalQueries(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.CreateMetric(ctx, Definition{Name: "budget.hot", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	points := make([]Point, 2101)
	for i := range points {
		points[i] = Point{MetricName: "budget.hot", EntityID: "n1", Timestamp: base.Add(time.Duration(i) * time.Second), Value: float64(i)}
	}
	if err := store.WriteBatch(ctx, points); err != nil {
		t.Fatal(err)
	}
	query := Query{MetricName: "budget.hot", EntityID: "n1", Start: base, End: points[len(points)-1].Timestamp, Limit: 2001, WorkBudget: 2000}
	if _, err := store.Query(ctx, query); !errors.Is(err, ErrQueryWorkBudgetExceeded) {
		t.Fatalf("expected hot budget rejection, got %v", err)
	}
	query.WorkBudget = 0
	got, err := store.Query(ctx, query)
	if err != nil || len(got) != 2001 {
		t.Fatalf("ordinary paged query changed: count=%d err=%v", len(got), err)
	}
	query.Limit = 0
	got, err = store.Query(ctx, query)
	if err != nil || len(got) != len(points) {
		t.Fatalf("ordinary unlimited query changed: count=%d err=%v", len(got), err)
	}
	query.WorkBudget = 2101
	got, err = store.Query(ctx, query)
	if err != nil || len(got) != len(points) {
		t.Fatalf("at budget query rejected: count=%d err=%v", len(got), err)
	}
	query.WorkBudget = -1
	if _, err = store.Query(ctx, query); err == nil {
		t.Fatal("negative work budget accepted")
	}
}
