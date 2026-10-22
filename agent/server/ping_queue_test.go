package server

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestPingQueueBoundsWorkersAndPendingTasks(t *testing.T) {
	q := newPingQueue(2, 3)
	defer q.close()
	release := make(chan struct{})
	defer close(release)
	var running, peak atomic.Int32
	job := func() {
		n := running.Add(1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		<-release
		running.Add(-1)
	}
	accepted := 0
	for i := 0; i < 100; i++ {
		if q.submit(job) {
			accepted++
		}
	}
	if accepted != 5 {
		t.Fatalf("accepted=%d, want 2 workers + 3 pending", accepted)
	}
	deadline := time.Now().Add(time.Second)
	for running.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if peak.Load() != 2 {
		t.Fatalf("peak concurrency=%d", peak.Load())
	}
}
