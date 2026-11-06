package menus

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

type fakeMenus struct {
	reads   chan chan []platform.MenuEntry
	invoked chan string
}

func (f *fakeMenus) Snapshot(ctx context.Context, _ platform.MenuSource) ([]platform.MenuEntry, error) {
	ch := make(chan []platform.MenuEntry, 1)
	f.reads <- ch
	return <-ch, nil
}
func (f *fakeMenus) Invoke(_ context.Context, _ platform.MenuSource, ref string) error {
	f.invoked <- ref
	return nil
}
func TestRefreshAndInvalidation(t *testing.T) {
	f := &fakeMenus{make(chan chan []platform.MenuEntry, 2), make(chan string, 1)}
	s := New(f)
	s.Capture(platform.MenuSource{PID: 1, Identity: "one"})
	done := make(chan error, 1)
	s.Refresh(context.Background(), time.Minute, func() { done <- s.State().Err })
	first := <-f.reads
	first <- []platform.MenuEntry{{Reference: "private", Title: "Open", Enabled: true}, {Reference: "disabled", Enabled: false}}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	items := s.Snapshot()
	if len(items) != 2 || items[0].Handle == "private" {
		t.Fatal(items)
	}
	if err := s.Invoke(context.Background(), items[1].Handle); !errors.Is(err, ErrInvalidHandle) {
		t.Fatal(err)
	}
	if err := s.Invoke(context.Background(), items[0].Handle); err != nil || <-f.invoked != "private" {
		t.Fatal(err)
	}
	s.Refresh(context.Background(), time.Minute, nil)
	old := <-f.reads
	s.Capture(platform.MenuSource{PID: 2, Identity: "two"})
	old <- []platform.MenuEntry{{Reference: "old", Enabled: true}}
	if err := s.Invoke(context.Background(), items[0].Handle); !errors.Is(err, ErrInvalidHandle) {
		t.Fatal(err)
	}
	if len(s.Snapshot()) != 0 {
		t.Fatal("old snapshot retained")
	}
}

type blockingMenus struct {
	started chan struct{}
	release chan struct{}
}

func (f *blockingMenus) Snapshot(context.Context, platform.MenuSource) ([]platform.MenuEntry, error) {
	return []platform.MenuEntry{{Reference: "ref", Enabled: true}}, nil
}

func (f *blockingMenus) Invoke(context.Context, platform.MenuSource, string) error {
	close(f.started)
	<-f.release
	return nil
}

func TestInvokeDoesNotBlockSnapshotOrCapture(t *testing.T) {
	f := &blockingMenus{make(chan struct{}), make(chan struct{})}
	s := New(f)
	s.Capture(platform.MenuSource{PID: 1, Identity: "one"})
	done := make(chan error, 1)
	s.Refresh(context.Background(), time.Minute, func() { done <- s.State().Err })
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	handle := s.Snapshot()[0].Handle
	invoked := make(chan error, 1)
	go func() { invoked <- s.Invoke(context.Background(), handle) }()
	<-f.started
	changed := make(chan struct{})
	go func() {
		_ = s.Snapshot()
		s.Capture(platform.MenuSource{PID: 2, Identity: "two"})
		close(changed)
	}()
	select {
	case <-changed:
	case <-time.After(time.Second):
		close(f.release)
		t.Fatal("snapshot or capture blocked on native invocation")
	}
	close(f.release)
	if err := <-invoked; err != nil {
		t.Fatal(err)
	}
	if err := s.Invoke(context.Background(), handle); !errors.Is(err, ErrInvalidHandle) {
		t.Fatal(err)
	}
}

type failedMenus struct{}

func (failedMenus) Snapshot(context.Context, platform.MenuSource) ([]platform.MenuEntry, error) {
	return nil, errors.New("previous source unavailable")
}

func (failedMenus) Invoke(context.Context, platform.MenuSource, string) error {
	return ErrInvalidHandle
}

func TestQueuedRefreshNotificationReadsCurrentSourceState(t *testing.T) {
	s := New(failedMenus{})
	s.Capture(platform.MenuSource{PID: 1, Identity: "one"})
	queued, deliver := make(chan struct{}), make(chan struct{})
	observed := make(chan SnapshotState, 1)
	s.Refresh(context.Background(), time.Minute, func() {
		close(queued)
		<-deliver
		observed <- s.State()
	})
	<-queued
	if s.State().Err == nil {
		t.Fatal("refresh failure was not stored for its source")
	}
	s.Capture(platform.MenuSource{PID: 2, Identity: "two"})
	close(deliver)
	state := <-observed
	if state.Err != nil || len(state.Items) != 0 {
		t.Fatalf("notification exposed the previous source: %+v", state)
	}
}
