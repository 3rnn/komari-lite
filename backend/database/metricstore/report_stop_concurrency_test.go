package metricstore

import (
	"context"
	v1 "github.com/komari-monitor/komari/protocol/v1"
	"testing"
	"time"
)

func TestConcurrentStopReportBatcherCompletesForEveryCaller(t *testing.T) {
	useReportTestStore(t, nil)
	worker := &reportBatchWorker{queue: make(chan v1.Report, reportBatchQueueSize), requests: make(chan reportBatchRequest), done: make(chan struct{})}
	reportBatcherMu.Lock()
	reportBatcher = worker
	reportBatcherMu.Unlock()
	// Let all callers capture the worker before starting its request receiver.
	done := make(chan error, 16)
	for i := 0; i < 16; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			done <- StopReportBatcher(ctx)
		}()
	}
	time.Sleep(10 * time.Millisecond)
	go worker.run()
	for i := 0; i < 16; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent stop = %v", err)
		}
	}
}
