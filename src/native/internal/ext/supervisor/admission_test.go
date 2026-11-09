package supervisor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

type admissionProbe struct {
	platform.WorkerProcess
	writes chan struct{}
}

type lifetimeProbe struct {
	entered chan struct{}
	release chan struct{}
}

type uninterruptibleProbe struct {
	entered chan struct{}
	release chan struct{}
}

func (p *uninterruptibleProbe) Serve(context.Context, protocol.ServiceCall) protocol.ServiceReply {
	close(p.entered)
	<-p.release
	return protocol.ServiceReply{}
}

func (p *lifetimeProbe) Serve(ctx context.Context, _ protocol.ServiceCall) protocol.ServiceReply {
	close(p.entered)
	<-ctx.Done()
	<-p.release
	return protocol.ServiceReply{}
}

func (p *admissionProbe) WriteLine([]byte) error {
	p.writes <- struct{}{}
	return nil
}

func TestAdmissionHonorsCancellationAndStoppedLifetime(t *testing.T) {
	r := New(Config{Clock: fake.NewClock()}, api.Identity{ID: "acme.test"}, "", nil)
	r.mu.Lock()
	r.state = StateStarting
	lifetime := r.lifetime
	r.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := r.childForCall(ctx, lifetime, protocol.MethodRun); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled admission: %v", err)
	}
	r.Stop()
	if _, _, err := r.childForCall(context.Background(), lifetime, protocol.MethodRun); err == nil {
		t.Fatal("admission crossed a stop")
	}
}

func TestCancelledDispatchGateDoesNotAdmit(t *testing.T) {
	gate := &dispatchGate{}
	gate.cancel()
	if err := gate.admit(context.Background(), context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled gate admitted request: %v", err)
	}
}

func TestCancelledQueuedCallNeverDispatches(t *testing.T) {
	r := New(Config{Clock: fake.NewClock()}, api.Identity{ID: "acme.test"}, "", nil)
	probe := &admissionProbe{writes: make(chan struct{}, 1)}
	r.mu.Lock()
	r.state, r.child = StateRunning, &childProcess{proc: probe}
	r.mu.Unlock()
	r.slots <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := r.Call(ctx, protocol.MethodRun, "", &api.Invocation{})
		done <- err
	}()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("queued call: %v", err)
	}
	<-r.slots
	select {
	case <-probe.writes:
		t.Fatal("cancelled call reached the child")
	default:
	}
}

func TestStopCancelsViewServiceContext(t *testing.T) {
	probe := &lifetimeProbe{entered: make(chan struct{}), release: make(chan struct{})}
	defer close(probe.release)
	r := New(Config{Clock: fake.NewClock()}, api.Identity{ID: "acme.test"}, "", probe)
	lifetime := r.lifetimeContext()
	done := make(chan error, 1)
	go func() {
		_, err := r.answerViewService(context.Background(), lifetime, nil, protocol.ServiceCall{}, protocol.MethodRun)
		done <- err
	}()
	<-probe.entered
	r.Stop()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("stopped service: %v", err)
	}
}

func TestCancelledServiceDoesNotStart(t *testing.T) {
	probe := &uninterruptibleProbe{entered: make(chan struct{}), release: make(chan struct{})}
	defer close(probe.release)
	r := New(Config{Clock: fake.NewClock()}, api.Identity{ID: "acme.test"}, "", probe)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.answerViewService(ctx, r.lifetimeContext(), nil, protocol.ServiceCall{}, protocol.MethodRun); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled service: %v", err)
	}
	select {
	case <-probe.entered:
		t.Fatal("cancelled service started")
	default:
	}
}

func TestStopReleasesUninterruptibleServiceWait(t *testing.T) {
	probe := &uninterruptibleProbe{entered: make(chan struct{}), release: make(chan struct{})}
	defer close(probe.release)
	r := New(Config{Clock: fake.NewClock()}, api.Identity{ID: "acme.test"}, "", probe)
	lifetime := r.lifetimeContext()
	done := make(chan error, 1)
	go func() {
		_, err := r.answerViewService(context.Background(), lifetime, nil, protocol.ServiceCall{}, protocol.MethodRun)
		done <- err
	}()
	<-probe.entered
	r.Stop()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("stopped service: %v", err)
	}
}

func TestStopEndsResponseWaitWithBlockedReader(t *testing.T) {
	r := New(Config{Clock: fake.NewClock()}, api.Identity{ID: "acme.test"}, "", nil)
	lifetime := r.lifetimeContext()
	child := &childProcess{lines: make(chan lineResult)}
	r.Stop()
	if _, err := r.awaitResponse(context.Background(), lifetime, child, protocol.Request{}, Budget{Hard: time.Hour}, nil); err == nil {
		t.Fatal("response wait survived stop")
	}
}
