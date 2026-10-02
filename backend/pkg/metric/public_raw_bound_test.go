package metric

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRelationalRawQueryRejectsMetadataBytesWithoutTruncation(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", "file:"+t.TempDir()+"/relational.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE metric_points(metric_name TEXT, entity_id TEXT, ts_nano INTEGER, value REAL, tags TEXT, labels TEXT, tags_hash TEXT DEFAULT '')`)
	if err != nil {
		t.Fatal(err)
	}
	store := &Store{db: db, dialect: sqliteDialect{}, tables: tables{points: "metric_points"}}
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	q := Query{MetricName: "cpu", EntityID: "n1", Start: stamp, End: stamp.Add(time.Second), Limit: 3, WorkBudget: 3}
	for i := 0; i < 2; i++ {
		if _, err = db.Exec(`INSERT INTO metric_points(metric_name,entity_id,ts_nano,value,tags,labels) VALUES ('cpu','n1',?,?,?,?)`, stamp.Add(time.Duration(i)*time.Second).UnixNano(), float64(i), `{}`, `{"v":"`+strings.Repeat("界", sqliteV4BudgetedPayloadBytes/6)+`"}`); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("cumulative UTF-8 label bytes: %d %v", len(got), err)
	}
	if _, err = db.Exec(`UPDATE metric_points SET labels='{}'`); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Query(ctx, q); err != nil || len(got) != 2 {
		t.Fatalf("small metadata: %d %v", len(got), err)
	}
	if _, err = db.Exec(`UPDATE metric_points SET tags=? WHERE ts_nano=?`, `{"v":"`+strings.Repeat("x", sqliteV4BudgetedPayloadBytes)+`"}`, stamp.UnixNano()); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("giant tags: %d %v", len(got), err)
	}
	q.WorkBudget = 0
	if got, err := store.Query(ctx, q); err != nil || len(got) != 2 {
		t.Fatalf("internal unrestricted read: %d %v", len(got), err)
	}
}

func TestRelationalRawQueryRejectsExcessRowsBeforeFetchingMetadata(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", "file:"+t.TempDir()+"/rows.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE metric_points(metric_name TEXT, entity_id TEXT, ts_nano INTEGER, value REAL, tags TEXT, labels TEXT, tags_hash TEXT DEFAULT '')`); err != nil {
		t.Fatal(err)
	}
	store := &Store{db: db, dialect: sqliteDialect{}, tables: tables{points: "metric_points"}}
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		if _, err := db.Exec(`INSERT INTO metric_points(metric_name,entity_id,ts_nano,value,tags,labels) VALUES ('cpu','n1',?,?,?,?)`, stamp.Add(time.Duration(i)*time.Second).UnixNano(), float64(i), `{}`, `{}`); err != nil {
			t.Fatal(err)
		}
	}
	q := Query{MetricName: "cpu", EntityID: "n1", Start: stamp, End: stamp.Add(4 * time.Second), Limit: 4, WorkBudget: 3}
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("excess rows must fail closed: %v %v", got, err)
	}
	q.WorkBudget = 4
	if got, err := store.Query(ctx, q); err != nil || len(got) != 4 {
		t.Fatalf("exact budget must return all rows: %v %v", got, err)
	}
}

func TestSQLiteV4HistoricalBlockPreflightIndexedAndBounded(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := store.CreateMetric(ctx, Definition{Name: "history.blocks", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, Point{MetricName: "history.blocks", EntityID: "n1", Timestamp: stamp, Value: 7}); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := store.db.QueryRow(`SELECT id FROM metric_series WHERE metric_name='history.blocks'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3000; i++ {
		start := stamp.Add(-time.Duration(3001-i) * time.Hour).UnixNano()
		if _, err := store.db.Exec(`INSERT INTO metric_point_blocks(series_id,start_nano,end_nano,point_count,codec,checksum,payload) VALUES (?,?,?,?,1,0,x'00')`, id, start, start, 1); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := store.db.Query(`EXPLAIN QUERY PLAN SELECT start_nano,end_nano,point_count,length(payload) FROM metric_point_blocks WHERE series_id=? AND end_nano>=? ORDER BY end_nano LIMIT ?`, id, stamp.UnixNano(), 3)
	if err != nil {
		t.Fatal(err)
	}
	detail := ""
	for rows.Next() {
		var a, b, c int
		var d string
		if err := rows.Scan(&a, &b, &c, &d); err != nil {
			t.Fatal(err)
		}
		detail += d
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if !strings.Contains(detail, "end_nano>?") || strings.Contains(detail, "TEMP B-TREE") {
		t.Fatalf("historical preflight must seek end index, not walk old blocks: %s", detail)
	}
	// The bounded indexed cursor visits zero of 3,000 old blocks.
	var candidates int
	candidateSQL := `SELECT count(*) FROM (SELECT end_nano FROM metric_point_blocks INDEXED BY metric_point_blocks_end_idx WHERE series_id=? AND end_nano>=? ORDER BY end_nano LIMIT 3)`
	if err := store.db.QueryRow(candidateSQL, id, stamp.UnixNano()).Scan(&candidates); err != nil || candidates != 0 {
		t.Fatalf("old blocks visited by candidate cursor: %d %v", candidates, err)
	}
	q := Query{MetricName: "history.blocks", EntityID: "n1", Start: stamp, End: stamp, WorkBudget: 2, Limit: 2}
	if got, err := store.Query(ctx, q); err != nil || len(got) != 1 || got[0].Value != 7 {
		t.Fatalf("historical range: %v %v", got, err)
	}
	for i := 0; i < 4; i++ {
		start := stamp.Add(time.Duration(i+1) * time.Hour).UnixNano()
		if _, err := store.db.Exec(`INSERT INTO metric_point_blocks(series_id,start_nano,end_nano,point_count,codec,checksum,payload) VALUES (?,?,?,?,1,0,x'00')`, id, start, start, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.db.QueryRow(candidateSQL, id, stamp.UnixNano()).Scan(&candidates); err != nil || candidates != 3 {
		t.Fatalf("candidate cursor should return only budget+1 of four future blocks: %d %v", candidates, err)
	}
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("too many future candidates must fail closed: %v %v", got, err)
	}
}
