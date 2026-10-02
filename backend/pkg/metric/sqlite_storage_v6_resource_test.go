package metric

import (
	"bytes"
	"compress/flate"
	"context"
	"errors"
	"hash/crc32"
	"testing"
	"time"
)

func TestSQLiteV6BudgetedSharedAxisAndValuesInflation(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLiteInDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	timestamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := store.CreateMetric(ctx, Definition{Name: "budget.shared", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, Point{MetricName: "budget.shared", EntityID: "n1", Timestamp: timestamp, Value: 7}); err != nil {
		t.Fatal(err)
	}
	var seriesID int64
	if err := store.db.QueryRowContext(ctx, `SELECT id FROM metric_series WHERE metric_name = 'budget.shared'`).Scan(&seriesID); err != nil {
		t.Fatal(err)
	}
	encoded, err := encodeSQLiteV6SharedPointBlock([]sqliteV4BlockPoint{{timestamp: timestamp.UnixNano(), valueBits: 0, labels: "{}", createdAt: 1}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.db.ExecContext(ctx, `INSERT INTO metric_point_axes (hash, codec, checksum, point_count, payload) VALUES (?, ?, ?, 1, ?)`, []byte("unique-axis"), encoded.axisCodec, int64(encoded.axisChecksum), encoded.axisPayload)
	if err != nil {
		t.Fatal(err)
	}
	axisID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO metric_point_blocks (series_id,start_nano,end_nano,point_count,codec,checksum,payload,axis_id) VALUES (?,?,?,?,?,?,?,?)`, seriesID, timestamp.UnixNano(), timestamp.UnixNano(), 1, encoded.codec, int64(encoded.checksum), encoded.payload, axisID); err != nil {
		t.Fatal(err)
	}
	query := Query{MetricName: "budget.shared", EntityID: "n1", Start: timestamp, End: timestamp, WorkBudget: 3}
	if points, err := store.Query(ctx, query); err != nil || len(points) != 1 || points[0].Value != 7 {
		t.Fatalf("normal shared block: %v %v", points, err)
	}
	bomb := func() []byte {
		var buffer bytes.Buffer
		buffer.WriteByte(sqliteV4PayloadDeflate)
		writer, err := flate.NewWriter(&buffer, flate.BestSpeed)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(bytes.Repeat([]byte{'x'}, sqliteV4BudgetedPayloadBytes+1)); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}()
	for _, column := range []string{"axis", "values"} {
		t.Run(column, func(t *testing.T) {
			if column == "axis" {
				if _, err := store.db.ExecContext(ctx, `UPDATE metric_point_axes SET payload=?, checksum=? WHERE id=?`, bomb, int64(crc32.ChecksumIEEE(bomb)), axisID); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := store.db.ExecContext(ctx, `UPDATE metric_point_axes SET payload=?, checksum=? WHERE id=?`, encoded.axisPayload, int64(encoded.axisChecksum), axisID); err != nil {
					t.Fatal(err)
				}
				if _, err := store.db.ExecContext(ctx, `UPDATE metric_point_blocks SET payload=?, checksum=? WHERE series_id=?`, bomb, int64(crc32.ChecksumIEEE(bomb)), seriesID); err != nil {
					t.Fatal(err)
				}
			}
			if points, err := store.Query(ctx, query); !errors.Is(err, ErrQueryWorkBudgetExceeded) || len(points) != 0 {
				t.Fatalf("compressed expansion should fail closed: %v %v", points, err)
			}
		})
	}
}
