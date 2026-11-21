package menushost

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/menus"
	"github.com/pablopunk/nvm/native/internal/platform"
)

type Source interface {
	platform.CurrentAppMenus
	Capture(context.Context, int) (platform.MenuSource, error)
}

type Adapter struct {
	mu                      sync.Mutex
	parent                  context.Context
	source                  Source
	service                 *menus.Service
	budget                  time.Duration
	notify                  func()
	cancel                  context.CancelFunc
	generation              uint64
	active, loading, closed bool
	err                     error
}

func New(parent context.Context, source Source, budget time.Duration, notify func()) *Adapter {
	return &Adapter{parent: parent, source: source, service: menus.New(source), budget: budget, notify: notify}
}

func (a *Adapter) signal() {
	if a.notify != nil {
		a.notify()
	}
}

// Capture binds the original host-supplied PID without waiting for OS reads.
func (a *Adapter) Capture(pid int) {
	a.mu.Lock()
	a.invalidateLocked()
	if a.closed || a.source == nil || a.parent.Err() != nil {
		a.mu.Unlock()
		a.signal()
		return
	}
	a.active, a.loading = true, true
	generation := a.generation
	ctx, cancel := context.WithCancel(a.parent)
	a.cancel = cancel
	a.mu.Unlock()
	a.signal()
	go a.capture(ctx, generation, pid)
}

func (a *Adapter) capture(ctx context.Context, generation uint64, pid int) {
	readCtx, cancel := context.WithTimeout(ctx, a.budget)
	source, err := a.source.Capture(readCtx, pid)
	if err == nil {
		err = readCtx.Err()
	}
	cancel()
	a.mu.Lock()
	if generation != a.generation || ctx.Err() != nil {
		a.mu.Unlock()
		return
	}
	if err != nil || source.PID != pid || source.Identity == "" {
		a.loading = false
		if err == nil {
			err = api.ErrNotAvailable
		}
		a.err = menuAPIError(err)
		a.mu.Unlock()
		a.signal()
		return
	}
	a.service.Capture(source)
	a.service.RequestRefresh(ctx, a.budget, func() { a.refreshed(generation) })
	a.mu.Unlock()
}

func (a *Adapter) refreshed(generation uint64) {
	a.mu.Lock()
	if generation != a.generation || !a.active || a.parent.Err() != nil {
		a.mu.Unlock()
		return
	}
	a.loading = false
	a.err = menuAPIError(a.service.State().Err)
	a.mu.Unlock()
	a.signal()
}

func (a *Adapter) invalidateLocked() {
	a.generation++
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.service.Dismiss()
	a.active, a.loading, a.err = false, false, nil
}

func (a *Adapter) Dismiss() { a.mu.Lock(); a.invalidateLocked(); a.mu.Unlock(); a.signal() }
func (a *Adapter) Close() {
	a.mu.Lock()
	a.closed = true
	a.invalidateLocked()
	a.mu.Unlock()
	a.signal()
}

func (a *Adapter) Snapshot(ctx context.Context) (api.MenuSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return api.MenuSnapshot{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.active || a.closed || a.parent.Err() != nil {
		return api.MenuSnapshot{}, api.ErrNotAvailable
	}
	state := a.service.State()
	result := api.MenuSnapshot{Loading: a.loading, Items: make([]api.MenuItem, 0, len(state.Items))}
	for _, item := range state.Items {
		result.Items = append(result.Items, api.MenuItem{Handle: item.Handle, Title: item.Title, Path: item.Path, Shortcut: item.Shortcut, Enabled: item.Enabled, Checked: item.Checked})
	}
	return result, a.err
}

func (a *Adapter) Invoke(ctx context.Context, handle string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.mu.Lock()
	if !a.active || a.closed || a.parent.Err() != nil {
		a.mu.Unlock()
		return api.ErrNotAvailable
	}
	// Service claims a handle itself; no adapter lock is held across native invocation.
	a.mu.Unlock()
	err := a.service.Invoke(ctx, handle)
	if errors.Is(err, menus.ErrInvalidHandle) {
		return api.ErrNotFound
	}
	return menuAPIError(err)
}

func menuAPIError(err error) error {
	var native *platform.WindowError
	if errors.As(err, &native) && native.Code == platform.WindowErrorDenied {
		return api.ErrPermission
	}
	return err
}
