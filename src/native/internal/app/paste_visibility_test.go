package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

type pasteUIQueue chan func()

func (q pasteUIQueue) Update(fn func()) { q <- fn }

type leasePastePort struct{ calls chan string }

func (f *leasePastePort) PasteIntoFrontmost(string) error { return nil }
func (f *leasePastePort) UndoInFrontmost(string) error { return nil }
func (f *leasePastePort) FrontmostAppID() (string, error) { return "", nil }
func (f *leasePastePort) PasteIntoFrontmostContext(ctx context.Context, _ string) error { f.calls <- "paste"; return ctx.Err() }
func (f *leasePastePort) UndoInFrontmostContext(ctx context.Context, _ string) error { f.calls <- "undo"; return ctx.Err() }
func (f *leasePastePort) TypeIntoFrontmost(string, string, time.Duration) error { return nil }
func (f *leasePastePort) TypeIntoFrontmostContext(ctx context.Context, _, _ string, _ time.Duration) error { f.calls <- "type"; return ctx.Err() }

func newLeasePaster() (*leasedPaster, *App) {
	app := &App{}
	return &leasedPaster{app: app, inner: &leasePastePort{calls: make(chan string, 8)}}, app
}

func TestLeasedPasterAllowsRepeatedHiddenPaste(t *testing.T) {
	p, _ := newLeasePaster()
	for range 3 {
		if err := p.PasteIntoFrontmost("app"); err != nil { t.Fatal(err) }
	}
	if len(p.leases) != 0 { t.Fatalf("leases remain: %d", len(p.leases)) }
}

func TestLeasedPasterRefusesVisiblePalette(t *testing.T) {
	p, app := newLeasePaster()
	app.Palette = goldenPalette(t).Palette
	app.Attach(&testWindow{visible: true})
	if err := p.PasteIntoFrontmost("app"); !errors.Is(err, platform.ErrConflict) { t.Fatalf("got %v", err) }
	if len(p.leases) != 0 { t.Fatalf("leases remain: %d", len(p.leases)) }
}

func TestLeasedPasterCancellationWhileUICallbackQueued(t *testing.T) {
	p, app := newLeasePaster()
	queue := make(pasteUIQueue, 1); app.uiQueue = queue
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- p.PasteIntoFrontmostContext(ctx, "app") }()
	callback := <-queue
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) { t.Fatalf("got %v", err) }
	callback()
	if len(p.leases) != 0 { t.Fatalf("leases remain: %d", len(p.leases)) }
}

func TestLeasedPasterRevokedBeforeDispatchSendsNoKeys(t *testing.T) {
	p, app := newLeasePaster()
	queue := make(pasteUIQueue, 1); app.uiQueue = queue
	result := make(chan error, 1)
	go func() { result <- p.PasteIntoFrontmost("app") }()
	callback := <-queue
	callback() // lease granted; fake OS dispatch is still waiting for caller.
	p.revoke()
	if err := <-result; !errors.Is(err, context.Canceled) { t.Fatalf("got %v", err) }
	if len(p.leases) != 0 { t.Fatalf("leases remain: %d", len(p.leases)) }
}

func TestLeasedPasterForwardsTypeAndUndo(t *testing.T) {
	p, _ := newLeasePaster()
	if err := p.TypeIntoFrontmostContext(context.Background(), "app", "x", 0); err != nil { t.Fatal(err) }
	if got := (<-p.inner.(*leasePastePort).calls); got != "type" { t.Fatalf("got %s", got) }
	if err := p.UndoInFrontmostContext(context.Background(), "app"); err != nil { t.Fatal(err) }
	if got := (<-p.inner.(*leasePastePort).calls); got != "undo" { t.Fatalf("got %s", got) }
}

func TestOldLeaseCleanupPreservesNewLease(t *testing.T) {
	p, _ := newLeasePaster()
	p.mu.Lock()
	p.leases = map[uint64]context.CancelFunc{1: func() {}, 2: func() {}}
	p.mu.Unlock()
	p.removeLease(1)
	if len(p.leases) != 1 || p.leases[2] == nil {
		t.Fatalf("new lease was removed: %#v", p.leases)
	}
}
