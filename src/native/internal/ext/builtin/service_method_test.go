package builtin

import (
	"context"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// deleting deletes a note from both a palette action and a search, the shape of the Notes built-in's delete action.
type deleting struct{}

func (deleting) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.deleting", Title: "Deleting"}}
}

func (deleting) RunAction(ctx api.Context, _ api.Invocation) (api.Result, error) {
	return api.Result{}, ctx.Notes().Delete("note")
}

func (deleting) SearchItems(ctx api.Context, _ string) ([]api.Item, error) {
	return nil, ctx.Notes().Delete("note")
}

// TestBuiltInServicesSeeTheMethodTheyServe checks that a built-in's service call carries the method of the call that
// made it, as a child's does, so a service that runs only from a palette action (deleting a note) accepts a run and
// refuses a search.
func TestBuiltInServicesSeeTheMethodTheyServe(t *testing.T) {
	var mu sync.Mutex
	var methods []string
	r := New(deleting{}, real.NewPorts().Clock, func(ctx context.Context, _ protocol.ServiceCall) protocol.ServiceReply {
		mu.Lock()
		defer mu.Unlock()
		methods = append(methods, protocol.RequestMethodOf(ctx))
		return protocol.ServiceReply{}
	})
	if _, err := r.Call(context.Background(), protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "delete"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Call(context.Background(), protocol.MethodSearch, "q", nil); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(methods) != 2 || methods[0] != protocol.MethodRun || methods[1] != protocol.MethodSearch {
		t.Fatalf("service calls saw methods %q, want run then search", methods)
	}
}
