package supervisor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

type heldViewServices struct {
	entered chan struct{}
	release chan struct{}
}

func (h *heldViewServices) Serve(context.Context, protocol.ServiceCall) protocol.ServiceReply {
	h.entered <- struct{}{}
	<-h.release
	return protocol.ServiceReply{}
}

func TestCanceledViewBoundsUninterruptibleHostCalls(t *testing.T) {
	host := &heldViewServices{entered: make(chan struct{}, 4), release: make(chan struct{})}
	r := New(Config{}, api.Identity{ID: "acme.test", Title: "test"}, "", host)
	defer close(host.release)
	for i := 0; i < 4; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := r.answerViewService(ctx, nil, protocol.ServiceCall{}, protocol.MethodRun)
			done <- err
		}()
		select {
		case <-host.entered:
		case <-time.After(time.Second):
			t.Fatal("service did not start")
		}
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("cancel blocked")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.answerViewService(ctx, nil, protocol.ServiceCall{}, protocol.MethodRun); !errors.Is(err, context.Canceled) {
		t.Fatalf("bounded call: %v", err)
	}
	select {
	case <-host.entered:
		t.Fatal("started more than four blocked calls")
	default:
	}
}
