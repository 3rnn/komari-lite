package metricstore

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/metric"
)

func TestBatchedEntityHistoryMatchesSerial(t *testing.T) {
	for _, compacted := range []bool{false, true} {
		t.Run(fmt.Sprintf("compacted-%t", compacted), func(t *testing.T) {
			ctx := context.Background()
			policy := defaultRollupPolicy()
			s := useReportTestStore(t, &policy)
			now := time.Now().UTC().Truncate(time.Minute)
			end := now.Add(-time.Hour)
			start := end.Add(-time.Hour)
			ids := make([]string, 0, 66)
			var points []metric.Point
			for i := 0; i < 65; i++ {
				id := fmt.Sprintf("history-%02d", i)
				ids = append(ids, id)
				for j := 0; j < 35+i%7; j++ {
					ts := start.Add(time.Duration(j*60+i%13) * time.Second)
					for _, name := range []string{MetricCPU, MetricNetTotalUp, MetricTrafficUp} {
						points = append(points, metric.Point{MetricName: name, EntityID: id, Timestamp: ts, Value: float64(j + i*100)})
					}
				}
			}
			if err := s.WriteBatch(ctx, points); err != nil {
				t.Fatal(err)
			}
			if compacted {
				if _, err := s.Compact(ctx, now); err != nil {
					t.Fatal(err)
				}
			}
			names := recordMetricNamesForLoadType("all")
			maxPoints := recordPerEntityMaxPoints(128, len(ids))
			ids = append(ids, "missing")
			batch, err := getRecordsByClientsAndTimeFromSeries(ctx, s, ids, start, end, names, maxPoints)
			if err != nil {
				t.Fatal(err)
			}
			var serial []models.Record
			for _, id := range ids {
				records, err := getRecordsByClientAndTimeFromSeries(ctx, s, id, start, end, names, maxPoints)
				if err != nil {
					t.Fatal(err)
				}
				serial = append(serial, records...)
			}
			sortRecords(serial)
			if len(serial) == 0 || !reflect.DeepEqual(serial, batch) {
				t.Fatal("batch changed per-entity historical records")
			}
			all, err := GetRecordsByTimeForLoadTypeMaxPoints(ctx, start, end, "all", 128)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(serial, all) {
				t.Fatal("32-entity chunks changed all-client historical records")
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			if _, err := getRecordsByClientsAndTimeFromSeries(canceled, s, ids, start, end, names, maxPoints); err == nil {
				t.Fatal("canceled batched read succeeded")
			}
			t.Logf("65 entities, compacted=%t: %d history records match serial reads exactly", compacted, len(all))
		})
	}
}
