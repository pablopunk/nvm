package files

import "sync"

// workQueue hands folders to scan workers. A worker takes one folder, reads it and pushes its subfolders back.
// The queue closes itself once no folder is waiting and no worker is busy, or when close is called (on
// cancellation or when the entry cap is reached), so every waiting worker wakes and returns.
type workQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []scanStart
	active int
	closed bool
}

// newWorkQueue returns an open, empty queue.
func newWorkQueue() *workQueue {
	q := &workQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// push adds a folder to scan. Pushes after close are dropped.
func (q *workQueue) push(item scanStart) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.items = append(q.items, item)
	q.cond.Signal()
}

// pop waits for a folder and takes it. It reports false once the queue is closed.
func (q *workQueue) pop() (scanStart, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if q.closed {
		return scanStart{}, false
	}
	last := len(q.items) - 1
	item := q.items[last]
	q.items = q.items[:last]
	q.active++
	return item, true
}

// done marks one popped folder as finished and closes the queue when that was the last outstanding work.
func (q *workQueue) done() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.active--
	if q.active == 0 && len(q.items) == 0 {
		q.closed = true
		q.cond.Broadcast()
	}
}

// close stops the queue: pending folders are dropped and every waiting worker returns.
func (q *workQueue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.items = nil
	q.cond.Broadcast()
}
