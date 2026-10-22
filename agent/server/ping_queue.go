package server

import "sync"

// Admission covers executing work and queued work, including result reporting.
// Saturation drops a task instead of spawning a goroutine to wait for a slot.
type pingQueue struct {
	jobs  chan func()
	slots chan struct{}
}

func newPingQueue(workers, pending int) *pingQueue {
	q := &pingQueue{jobs: make(chan func(), workers+pending), slots: make(chan struct{}, workers+pending)}
	for i := 0; i < workers; i++ {
		go func() {
			for job := range q.jobs {
				job()
				<-q.slots
			}
		}()
	}
	return q
}
func (q *pingQueue) submit(job func()) bool {
	select {
	case q.slots <- struct{}{}:
		q.jobs <- job
		return true
	default:
		return false
	}
}
func (q *pingQueue) close() { close(q.jobs) }

var pingQueueOnce sync.Once
var taskPingQueue *pingQueue

func submitPing(job func()) bool {
	pingQueueOnce.Do(func() { taskPingQueue = newPingQueue(8, 32) })
	return taskPingQueue.submit(job)
}
