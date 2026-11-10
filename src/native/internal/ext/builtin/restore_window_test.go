package builtin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// restoringSample is a built-in that restores one persistent window and no other.
type restoringSample struct{}

func (restoringSample) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.restoring", Title: "Restoring"}}
}

func (restoringSample) RestoreWindow(_ api.Context, key string) (*api.WindowRestore, error) {
	if key != "ticker.floating" {
		return nil, nil
	}
	return &api.WindowRestore{View: api.View{ID: "ticker", Kind: api.ViewList, Title: "Ticker"}}, nil
}

// TestRestoreWindowAnswersWithTheFreshViewOrNull checks the restore method reaches RestoreWindow with its key, and that
// a key the extension no longer restores answers null, which purges it on the host.
func TestRestoreWindowAnswersWithTheFreshViewOrNull(t *testing.T) {
	runner := New(restoringSample{}, fake.NewClock(), nil)
	if !runner.Provides(protocol.MethodRestoreWindow) {
		t.Fatal("an extension with RestoreWindow does not provide the restore method")
	}
	restored, err := runner.Call(context.Background(), protocol.MethodRestoreWindow, "", &api.Invocation{Ref: api.ActionRef{ID: "ticker.floating"}})
	if err != nil || restored.Failure != nil {
		t.Fatalf("restore failed: %v %+v", err, restored.Failure)
	}
	var window api.WindowRestore
	if err := json.Unmarshal(restored.Data, &window); err != nil || window.View.Title != "Ticker" {
		t.Fatalf("restored view = %s (%v), want the ticker view", restored.Data, err)
	}
	gone, err := runner.Call(context.Background(), protocol.MethodRestoreWindow, "", &api.Invocation{Ref: api.ActionRef{ID: "ticker.gone"}})
	if err != nil || string(gone.Data) != "null" {
		t.Fatalf("restore of a retired key = %s (%v), want null", gone.Data, err)
	}
}
