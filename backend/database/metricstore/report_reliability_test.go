package metricstore

import (
	"context"
	"errors"
	"github.com/komari-monitor/komari/pkg/metric"
	v1 "github.com/komari-monitor/komari/protocol/v1"
	"testing"
	"time"
)

func TestFailedFlushKeepsBoundedReportsAndBackpressure(t *testing.T) {
	s := useReportTestStore(t, nil)
	StartReportBatcher()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for round := 0; round < 3; round++ {
		for i := 0; i < reportBatchQueueSize; i++ {
			_, _ = WriteReport(context.Background(), v1.Report{UUID: "bounded", UpdatedAt: time.Now()})
		}
		// A canceled flush must retain reports, not free capacity indefinitely.
		req := reportBatchRequest{ctx: ctx, done: make(chan error, 1)}
		reportBatcher.requests <- req
		if err := <-req.done; !errors.Is(err, context.Canceled) {
			t.Fatalf("flush = %v", err)
		}
	}
	if _, err := WriteReport(context.Background(), v1.Report{UUID: "bounded", UpdatedAt: time.Now()}); !errors.Is(err, ErrReportBatchQueueFull) {
		t.Fatalf("failure released queue budget: %v", err)
	}
	if err := StopReportBatcher(context.Background()); err != nil {
		t.Fatal(err)
	}
	points, err := s.Query(context.Background(), metric.Query{MetricName: MetricCPU, EntityID: "bounded", Start: time.Now().Add(-time.Hour), End: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != reportBatchQueueSize {
		t.Fatalf("recovery persisted %d of %d accepted reports", len(points), reportBatchQueueSize)
	}
}

func TestFailedBatchDoesNotMutatePendingReportsWhileFilteringDeletedEntities(t *testing.T) {
	useReportTestStore(t, nil)
	BlockEntityWrites("deleted-retry")
	defer UnblockEntityWrites("deleted-retry")
	pending := []v1.Report{{UUID: "deleted-retry"}, {UUID: "retained-a"}, {UUID: "retained-b"}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := writePendingReports(ctx, &pending); !errors.Is(err, context.Canceled) {
		t.Fatalf("batch error = %v", err)
	}
	if pending[0].UUID != "deleted-retry" || pending[1].UUID != "retained-a" || pending[2].UUID != "retained-b" {
		t.Fatalf("failed batch corrupted retry reports: %#v", pending)
	}
}

func TestStorageFailurePreservesReportsUntilRecovery(t *testing.T) {
	s := useReportTestStore(t, nil)
	StartReportBatcher()
	defer func() { storeMu.Lock(); store = s; storeMu.Unlock(); _ = StopReportBatcher(context.Background()) }()
	for i := 0; i < 300; i++ {
		if _, err := WriteReport(context.Background(), v1.Report{UUID: "storage-recovery", UpdatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	storeMu.Lock()
	store = nil
	storeMu.Unlock()
	for i := 0; i < 3; i++ {
		if err := FlushReportBatch(context.Background()); err == nil {
			t.Fatal("store outage flush unexpectedly succeeded")
		}
	}
	reportBatcher.mu.Lock()
	outstanding := reportBatcher.outstanding
	reportBatcher.mu.Unlock()
	if outstanding != 300 {
		t.Fatalf("outage retained %d reports, want 300", outstanding)
	}
	storeMu.Lock()
	store = s
	storeMu.Unlock()
	if err := FlushReportBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	points, err := s.Query(context.Background(), metric.Query{MetricName: MetricCPU, EntityID: "storage-recovery", Start: time.Now().Add(-time.Hour), End: time.Now().Add(time.Hour)})
	if err != nil || len(points) != 300 {
		t.Fatalf("recovered points = %d, error = %v", len(points), err)
	}
}

func TestFailedStopRetainsWorkerForRetry(t *testing.T) {
	useReportTestStore(t, nil)
	StartReportBatcher()
	if _, err := WriteReport(context.Background(), v1.Report{UUID: "retry-stop", UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	worker := reportBatcher
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := reportBatchRequest{ctx: ctx, done: make(chan error, 1), stop: true}
	worker.requests <- req
	if err := <-req.done; !errors.Is(err, context.Canceled) {
		t.Fatalf("stop = %v", err)
	}
	select {
	case <-worker.done:
		t.Fatal("failed shutdown discarded pending reports")
	default:
	}
	if err := StopReportBatcher(context.Background()); err != nil {
		t.Fatal(err)
	}
}
