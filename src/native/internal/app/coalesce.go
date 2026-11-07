package app

import "sync/atomic"

// coalescer runs a refresh once for a burst of requests. A request made while a run is pending adds nothing: the
// pending run reads the state as it is when it runs, so no change is lost.
type coalescer struct {
	pending atomic.Bool
}

// request asks for run to happen on dispatch, unless a run is already pending. The pending flag clears before run starts,
// so a request made during run schedules another one.
func (c *coalescer) request(dispatch func(func()), run func()) {
	if !c.pending.CompareAndSwap(false, true) {
		return
	}
	dispatch(func() {
		c.pending.Store(false)
		run()
	})
}
