package safego

import (
	"context"
	"sync"
)

// Group runs named tasks together, like errgroup but panic-safe. The first task that fails cancels the
// context the other tasks receive; Wait returns that first error once every task has returned. Each task
// runs under Run, so panics, timeouts and restarts behave as they do for Run.
type Group struct {
	ctx    context.Context
	cancel context.CancelFunc
	opts   []Option
	wg     sync.WaitGroup
	once   sync.Once
	err    error
}

// NewGroup returns a group whose tasks share a context derived from the parent in opts.
func NewGroup(opts ...Option) *Group {
	o := resolve(opts)
	ctx, cancel := context.WithCancel(o.parent)
	return &Group{ctx: ctx, cancel: cancel, opts: opts}
}

// Go starts a task on its own goroutine. The task's context is the group's context.
func (g *Group) Go(name string, fn func(ctx context.Context) error) {
	g.wg.Add(1)
	taskOpts := append(append([]Option{}, g.opts...), WithContext(g.ctx))
	go func() {
		// Done runs after track has uncounted the goroutine, so Wait returning means Watching().Live is settled.
		defer g.wg.Done()
		track(name, func() {
			if err := Run(name, fn, taskOpts...); err != nil {
				g.fail(err)
			}
		})
	}()
}

// Wait blocks until every task has returned, releases the group's context and returns the first error.
func (g *Group) Wait() error {
	g.wg.Wait()
	g.cancel()
	return g.err
}

// fail records the first failure and cancels the group. Later failures are ignored.
func (g *Group) fail(err error) {
	g.once.Do(func() {
		g.err = err
		g.cancel()
	})
}
