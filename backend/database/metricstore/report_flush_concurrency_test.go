package metricstore

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/komari-monitor/komari/protocol/v1"
)

func TestFlushReportBatchReturnsWhenConcurrentStopExitsWorker(t *testing.T) {
	useReportTestStore(t, nil)
	worker := &reportBatchWorker{
		queue:    make(chan v1.Report, reportBatchQueueSize),
		requests: make(chan reportBatchRequest, 2),
		done:     make(chan struct{}),
	}
	reportBatcherMu.Lock()
	reportBatcher = worker
	reportBatcherMu.Unlock()
	t.Cleanup(func() {
		if err := StopReportBatcher(context.Background()); err != nil {
			t.Errorf("stop test worker: %v", err)
		}
	})
	// Model a stop queued after Flush checked stopping, but before its send.
	worker.requests <- reportBatchRequest{ctx: context.Background(), done: make(chan error, 1), stop: true}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- FlushReportBatch(ctx) }()
	deadline := time.After(time.Second)
	for len(worker.requests) != 2 {
		select {
		case <-deadline:
			t.Fatal("flush did not queue its request")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	go worker.run()
	<-worker.done
	select {
	case err := <-result:
		if !errors.Is(err, ErrReportBatchStopped) {
			t.Fatalf("flush after worker exit = %v, want ErrReportBatchStopped", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("flush waited for an abandoned request after concurrent stop")
	}
}
