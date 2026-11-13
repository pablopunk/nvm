package menushost

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

type captureCall struct {
	pid   int
	ctx   context.Context
	reply chan platform.MenuSource
}
type readCall struct {
	ctx    context.Context
	source platform.MenuSource
	reply  chan []platform.MenuEntry
}
type fakeSource struct {
	captures chan captureCall
	reads    chan readCall
	invoked  chan string
}

func newFake() *fakeSource {
	return &fakeSource{make(chan captureCall, 10), make(chan readCall, 10), make(chan string, 10)}
}
func (f *fakeSource) Capture(ctx context.Context, pid int) (platform.MenuSource, error) {
	c := captureCall{pid, ctx, make(chan platform.MenuSource, 1)}
	f.captures <- c
	select {
	case s := <-c.reply:
		return s, nil
	case <-ctx.Done():
		return platform.MenuSource{}, ctx.Err()
	}
}
func (f *fakeSource) Snapshot(ctx context.Context, source platform.MenuSource) ([]platform.MenuEntry, error) {
	c := readCall{ctx, source, make(chan []platform.MenuEntry, 1)}
	f.reads <- c
	select {
	case v := <-c.reply:
		return v, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (f *fakeSource) Invoke(ctx context.Context, source platform.MenuSource, ref string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.invoked <- source.Identity + ":" + ref
	return nil
}
func TestSnapshotCachedAndSourceLifetime(t *testing.T) {
	f := newFake()
	changes := make(chan struct{}, 10)
	a := New(context.Background(), f, time.Minute, func() { changes <- struct{}{} })
	defer a.Close()
	a.Capture(42)
	<-changes
	c := <-f.captures
	if c.pid != 42 {
		t.Fatal(c.pid)
	}
	ctx, cancel := context.WithCancel(context.Background())
	for i := 0; i < 10; i++ {
		s, err := a.Snapshot(ctx)
		if err != nil || !s.Loading {
			t.Fatal(s, err)
		}
	}
	cancel()
	select {
	case <-c.ctx.Done():
		t.Fatal("RPC cancelled capture")
	default:
	}
	c.reply <- platform.MenuSource{PID: 42, Identity: "identity"}
	read := <-f.reads
	select {
	case <-read.ctx.Done():
		t.Fatal("RPC cancelled read")
	default:
	}
	for i := 0; i < 10; i++ {
		_, _ = a.Snapshot(context.Background())
	}
	select {
	case <-f.captures:
		t.Fatal("duplicate capture")
	case <-f.reads:
		t.Fatal("duplicate read")
	default:
	}
	read.reply <- []platform.MenuEntry{{Reference: "private", Title: "Open", Path: []string{"File"}, Enabled: true}}
	<-changes
	s, err := a.Snapshot(context.Background())
	if err != nil || s.Loading || len(s.Items) != 1 || s.Items[0].Handle == "private" {
		t.Fatal(s, err)
	}
	if err = a.Invoke(context.Background(), s.Items[0].Handle); err != nil || <-f.invoked != "identity:private" {
		t.Fatal(err)
	}
}
func TestGenerationAndClose(t *testing.T) {
	f := newFake()
	changes := make(chan struct{}, 20)
	a := New(context.Background(), f, time.Minute, func() { changes <- struct{}{} })
	a.Capture(1)
	<-changes
	old := <-f.captures
	a.Capture(2)
	<-changes
	current := <-f.captures
	old.reply <- platform.MenuSource{PID: 1, Identity: "old"}
	current.reply <- platform.MenuSource{PID: 2, Identity: "new"}
	read := <-f.reads
	read.reply <- []platform.MenuEntry{{Reference: "ref", Enabled: true}}
	<-changes
	s, err := a.Snapshot(context.Background())
	if err != nil || len(s.Items) != 1 {
		t.Fatal(s, err)
	}
	handle := s.Items[0].Handle
	a.Capture(3)
	<-changes
	held := <-f.captures
	if err := a.Invoke(context.Background(), handle); !errors.Is(err, api.ErrNotFound) {
		t.Fatal(err)
	}
	a.Dismiss()
	<-changes
	select {
	case <-held.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("dismiss did not cancel")
	}
	a.Capture(4)
	<-changes
	c := <-f.captures
	c.reply <- platform.MenuSource{PID: 4, Identity: "four"}
	pending := <-f.reads
	a.Close()
	<-changes
	select {
	case <-pending.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("close did not cancel")
	}
	if _, err := a.Snapshot(context.Background()); !errors.Is(err, api.ErrNotAvailable) {
		t.Fatal(err)
	}
}
func TestErrorsAndReentrantNotification(t *testing.T) {
	a := New(context.Background(), nil, time.Minute, nil)
	if _, err := a.Snapshot(context.Background()); !errors.Is(err, api.ErrNotAvailable) {
		t.Fatal(err)
	}
	if err := a.Invoke(context.Background(), "x"); !errors.Is(err, api.ErrNotAvailable) {
		t.Fatal(err)
	}
	f := newFake()
	notified := make(chan struct{}, 10)
	var b *Adapter
	b = New(context.Background(), f, time.Minute, func() { _, _ = b.Snapshot(context.Background()); notified <- struct{}{} })
	b.Capture(5)
	<-notified
	c := <-f.captures
	c.reply <- platform.MenuSource{PID: 5, Identity: "five"}
	r := <-f.reads
	r.reply <- nil
	<-notified
	if err := b.Invoke(context.Background(), "unknown"); !errors.Is(err, api.ErrNotFound) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.Invoke(ctx, "unknown"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := b.Snapshot(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	b.Close()
}
