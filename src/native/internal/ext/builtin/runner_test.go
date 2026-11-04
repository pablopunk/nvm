package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// sample is a built-in that answers every method it can.
type sample struct {
	calls []string
	mu    sync.Mutex
	// storeKey, when set, makes SearchItems write through the storage service.
	storeKey string
}

func (s *sample) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.sample", Title: "Sample"}}
}

func (s *sample) Commands(api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "open", Title: "Open Sample"}}, nil
}

func (s *sample) RootItems(api.Context) ([]api.Item, error) {
	return []api.Item{{ID: "root", Title: "Root row"}}, nil
}

func (s *sample) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if s.storeKey != "" {
		if err := ctx.Storage().Set(s.storeKey, query); err != nil {
			return nil, err
		}
	}
	if query == "boom" {
		panic("boom in SearchItems")
	}
	return []api.Item{{ID: "hit", Title: "Hit " + query}}, nil
}

func (s *sample) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	return api.Result{Next: &api.Action{Kind: api.ActionRun, Ref: &in.Ref}}, nil
}

// bare implements only Extension: every method is unavailable.
type bare struct{}

func (bare) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.bare", Title: "Bare"}}
}

func noServices(protocol.ServiceCall) protocol.ServiceReply { return protocol.ServiceReply{} }

func newSample(t *testing.T, ext api.Extension, serve func(protocol.ServiceCall) protocol.ServiceReply) *Runner {
	t.Helper()
	if serve == nil {
		serve = noServices
	}
	return New(ext, real.NewPorts().Clock, serve)
}

func TestProvidesReportsOnlyTheInterfacesTheExtensionImplements(t *testing.T) {
	r := newSample(t, &sample{}, nil)
	for _, method := range []string{protocol.MethodCommands, protocol.MethodRoot, protocol.MethodSearch, protocol.MethodRun} {
		if !r.Provides(method) {
			t.Errorf("Provides(%s) = false, want true", method)
		}
	}
	if r.Provides(protocol.MethodLoad) || r.Provides(protocol.MethodMenuBars) {
		t.Errorf("sample does not implement load or menu bars")
	}
	if newSample(t, bare{}, nil).Provides(protocol.MethodSearch) {
		t.Errorf("a bare extension provides no search")
	}
}

func TestSearchAnswersWithTheItemsAsJSON(t *testing.T) {
	r := newSample(t, &sample{}, nil)
	response, err := r.Call(context.Background(), protocol.MethodSearch, "two", nil)
	if err != nil || response.Failure != nil {
		t.Fatalf("Call: %v, failure %+v", err, response.Failure)
	}
	var items []api.Item
	if err := json.Unmarshal(response.Data, &items); err != nil {
		t.Fatalf("items are not JSON: %v", err)
	}
	if len(items) != 1 || items[0].Title != "Hit two" {
		t.Fatalf("items = %+v", items)
	}
}

func TestMissingInterfaceIsAnUnavailableFailure(t *testing.T) {
	response, err := newSample(t, bare{}, nil).Call(context.Background(), protocol.MethodSearch, "x", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if response.Failure == nil || response.Failure.Code != api.CodeUnavailable {
		t.Fatalf("failure = %+v, want unavailable", response.Failure)
	}
}

func TestPanicBecomesAnInternalFailureWithAReference(t *testing.T) {
	response, err := newSample(t, &sample{}, nil).Call(context.Background(), protocol.MethodSearch, "boom", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if response.Failure == nil || response.Failure.Code != api.CodeInternal || response.Failure.Reference == "" {
		t.Fatalf("failure = %+v, want internal with a reference", response.Failure)
	}
	if !strings.Contains(response.Failure.Message, "boom") {
		t.Errorf("message %q does not name the panic", response.Failure.Message)
	}
}

// stuck never returns from SearchItems, so only the caller's budget ends the call.
type stuck struct{ bare }

func (stuck) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.stuck", Title: "Stuck"}}
}

func (stuck) SearchItems(api.Context, string) ([]api.Item, error) {
	select {}
}

func TestBudgetEndsACallThatNeverAnswers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := newSample(t, stuck{}, nil).Call(ctx, protocol.MethodSearch, "x", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the deadline", err)
	}
}

func TestRunPassesTheInvocationAndReturnsTheResult(t *testing.T) {
	r := newSample(t, &sample{}, nil)
	response, err := r.Call(context.Background(), protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "open"}})
	if err != nil || response.Failure != nil {
		t.Fatalf("Call: %v, failure %+v", err, response.Failure)
	}
	var result api.Result
	if err := json.Unmarshal(response.Data, &result); err != nil {
		t.Fatalf("result is not JSON: %v", err)
	}
	if result.Next == nil || result.Next.Ref == nil || result.Next.Ref.ID != "open" {
		t.Fatalf("result = %+v, want the invoked action echoed as Next", result)
	}
}

// TestServiceCallsGoThroughTheHandlerInProcess shows that a built-in's Storage reaches the service handler directly
// and that the handler's answer is the one the extension sees.
func TestServiceCallsGoThroughTheHandlerInProcess(t *testing.T) {
	var got []protocol.ServiceCall
	serve := func(call protocol.ServiceCall) protocol.ServiceReply {
		got = append(got, call)
		return protocol.ServiceReply{}
	}
	r := newSample(t, &sample{storeKey: "last"}, serve)
	if _, err := r.Call(context.Background(), protocol.MethodSearch, "typed", nil); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if len(got) != 1 || got[0].Service != protocol.ServiceStorage || got[0].Op != protocol.OpSet {
		t.Fatalf("service calls = %+v, want one storage set", got)
	}
}

func TestLifecycleIsAlwaysRunning(t *testing.T) {
	r := newSample(t, &sample{}, nil)
	r.Stop()
	r.Reset()
	r.Start()
	if state := r.Status().State; state != "running" {
		t.Fatalf("state = %q, want running", state)
	}
}
