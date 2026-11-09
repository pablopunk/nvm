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
			_, err := r.answerViewService(ctx, r.lifetimeContext(), nil, protocol.ServiceCall{}, protocol.MethodRun)
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
	if _, err := r.answerViewService(ctx, r.lifetimeContext(), nil, protocol.ServiceCall{}, protocol.MethodRun); !errors.Is(err, context.Canceled) {
		t.Fatalf("bounded call: %v", err)
	}
	select {
	case <-host.entered:
		t.Fatal("started more than four blocked calls")
	default:
	}
}

func TestCancellationOwnsSerializedExchange(t *testing.T) {
	for _, view := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "initial-view"}[view], func(t *testing.T) {
			rig := newServicesRig(t)
			rig.deps.Home = t.TempDir()
			r := servicedRuntime(t, "local.screenshots", rig, fixtureManifest("local.screenshots", api.CapabilityDesktopFiles))
			blocker := &blockingScreenshotsService{ServiceHandler: r.services, entered: make(chan struct{}), release: make(chan struct{})}
			r.services = blocker
			released := false
			defer func() {
				if !released {
					close(blocker.release)
				}
			}()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := r.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "screenshots.grid"}, AwaitInitialView: view, Launch: api.LaunchContext{Refresh: true}})
				done <- err
			}()
			select {
			case <-blocker.entered:
			case <-time.After(waitTimeout):
				t.Fatal("service not entered")
			}
			holdStart := time.Now()
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel: %v", err)
				}
			case <-time.After(waitTimeout):
				t.Fatal("caller did not return")
			}
			if !view && len(r.slots) != 1 {
				t.Fatal("ordinary exchange released slot while service was held")
			}
			if view {
				select {
				case r.slots <- struct{}{}:
					<-r.slots
				case <-time.After(waitTimeout):
					t.Fatal("view exchange retained slot")
				}
			}
			close(blocker.release)
			released = true
			select {
			case r.slots <- struct{}{}:
				<-r.slots
				t.Logf("slot available %s after cancellation", time.Since(holdStart))
			case <-time.After(waitTimeout):
				t.Fatal("exchange did not release slot")
			}
			if view {
				response, err := r.Call(context.Background(), protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "screenshots.grid"}, AwaitInitialView: true})
				if err != nil || response.Failure != nil {
					t.Fatalf("reopen while child restarts: %v %+v", err, response.Failure)
				}
			}
		})
	}
}
