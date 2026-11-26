package child

import (
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// runHostlessFixtureCommand runs a fixture command whose host calls fail, as no host answers them, and returns its result.
// The output also holds the service calls the child sent, so the result is the response to the request's id.
func runHostlessFixtureCommand(t *testing.T, fixture, command string, args map[string]string) api.Result {
	t.Helper()
	_, responses := exchange(t, fixture, protocol.Request{ID: 7, Method: protocol.MethodRun, Invocation: &api.Invocation{Ref: api.ActionRef{ID: command, Args: args}}})
	for _, response := range responses {
		if response.ID != 7 {
			continue
		}
		if response.Failure != nil {
			t.Fatalf("%s/%s failed: %+v", fixture, command, response.Failure)
		}
		var result api.Result
		if err := json.Unmarshal(response.Data, &result); err != nil {
			t.Fatalf("decode result: %v", err)
		}
		return result
	}
	t.Fatalf("%s/%s: no response in %+v", fixture, command, responses)
	return api.Result{}
}

// Without a host the fixture's list of extensions is empty and shows its empty state; the review it opens is host UI.
func TestExtensionsFixtureShowsTheEmptyStateWithoutAHost(t *testing.T) {
	result := runHostlessFixtureCommand(t, "acme.extensions", "list", nil)
	if result.View == nil || result.View.Kind != api.ViewList || len(result.View.Items) != 0 || result.View.EmptyView == nil {
		t.Fatalf("view = %+v; want an empty list with an empty state", result.View)
	}
}

func TestExtensionsFixtureReportsARefusedReview(t *testing.T) {
	result := runHostlessFixtureCommand(t, "acme.extensions", "review", map[string]string{"id": "acme.weather"})
	if result.Toast == nil || result.Toast.Tone != "error" {
		t.Fatalf("result = %+v; want an error toast", result)
	}
}
