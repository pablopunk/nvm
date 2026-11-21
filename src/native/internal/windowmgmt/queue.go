package windowmgmt

import (
	"context"
	"sync"
)

// pidQueue runs tasks for one process one at a time, so two commands aimed at the same app never
// interleave their reads and writes. Tasks for different processes run in parallel.
type pidQueue struct {
	mu    sync.Mutex
	lanes map[int]*queueLane
}

type queueLane struct {
	turn  chan struct{}
	users int
}

func newPIDQueue() *pidQueue {
	return &pidQueue{lanes: map[int]*queueLane{}}
}

// runQueued waits for its turn on the process lane, then runs task. A cancelled context gives up the
// wait without running the task.
func runQueued[T any](ctx context.Context, queue *pidQueue, pid int, task func(context.Context) (T, error)) (T, error) {
	var zero T
	lane := queue.join(pid)
	defer queue.leave(pid)
	select {
	case lane.turn <- struct{}{}:
	case <-ctx.Done():
		return zero, ctx.Err()
	}
	defer func() { <-lane.turn }()
	return task(ctx)
}

func (q *pidQueue) join(pid int) *queueLane {
	q.mu.Lock()
	defer q.mu.Unlock()
	lane, ok := q.lanes[pid]
	if !ok {
		lane = &queueLane{turn: make(chan struct{}, 1)}
		q.lanes[pid] = lane
	}
	lane.users++
	return lane
}

func (q *pidQueue) leave(pid int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	lane := q.lanes[pid]
	lane.users--
	if lane.users == 0 {
		delete(q.lanes, pid)
	}
}
