package metric

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSQLiteV4BudgetedBlocksShareInflatedByteLimit(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := store.CreateMetric(ctx, Definition{Name: "budget.cumulative", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, Point{MetricName: "budget.cumulative", EntityID: "n1", Timestamp: start, Value: 3}); err != nil {
		t.Fatal(err)
	}
	var seriesID int64
	if err := store.db.QueryRowContext(ctx, `SELECT id FROM metric_series WHERE metric_name='budget.cumulative'`).Scan(&seriesID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		timestamp := start.Add(time.Duration(i) * time.Second).UnixNano()
		block, err := encodeSQLiteV4Block([]sqliteV4BlockPoint{{timestamp: timestamp, valueBits: 0, labels: strings.Repeat("x", sqliteV4BudgetedPayloadBytes/2), createdAt: 1}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.db.ExecContext(ctx, `INSERT INTO metric_point_blocks (series_id,start_nano,end_nano,point_count,codec,checksum,payload) VALUES (?,?,?,?,?,?,?)`, seriesID, timestamp, timestamp, 1, block.codec, int64(block.checksum), block.payload); err != nil {
			t.Fatal(err)
		}
	}
	q := Query{MetricName: "budget.cumulative", EntityID: "n1", Start: start, End: start.Add(time.Second), WorkBudget: 2}
	if got, err := store.Query(ctx, q); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(got) != 0 {
		t.Fatalf("combined inflation must reject full query: %v %v", got, err)
	}
}
