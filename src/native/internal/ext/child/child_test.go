package child

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

const fixtureRoot = "../testdata/extensions/"

// exchange runs the child loop in process over the given requests and returns the ready line and responses.
func exchange(t *testing.T, fixture string, requests ...protocol.Request) (protocol.Ready, []protocol.Response) {
	t.Helper()
	var input bytes.Buffer
	for _, req := range requests {
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		input.Write(append(data, '\n'))
	}
	var output bytes.Buffer
	serve(fixtureRoot+fixture+"/main.go", 0, api.Identity{ID: "acme.fixture", Title: "Fixture"}, &input, &output)
	lines := bufio.NewScanner(&output)
	var ready protocol.Ready
	if !lines.Scan() {
		t.Fatal("child wrote no ready line")
	}
	if err := json.Unmarshal(lines.Bytes(), &ready); err != nil {
		t.Fatalf("ready line: %v", err)
	}
	var responses []protocol.Response
	for lines.Scan() {
		var response protocol.Response
		if err := json.Unmarshal(lines.Bytes(), &response); err != nil {
			t.Fatalf("response line %q: %v", lines.Text(), err)
		}
		responses = append(responses, response)
	}
	return ready, responses
}

func TestGoodExtensionAnswersEachMethodWithJSON(t *testing.T) {
	ready, responses := exchange(t, "acme.good",
		protocol.Request{ID: 1, Method: protocol.MethodSearch, Query: "a"},
		protocol.Request{ID: 2, Method: protocol.MethodCommands},
		protocol.Request{ID: 3, Method: protocol.MethodRoot},
		protocol.Request{ID: 4, Method: protocol.MethodRun, Invocation: &api.Invocation{Ref: api.ActionRef{ID: "hello"}}},
	)
	if !ready.OK {
		t.Fatalf("ready = %+v; want OK", ready)
	}
	if len(responses) != 4 {
		t.Fatalf("got %d responses; want 4", len(responses))
	}
	var items []api.Item
	if err := json.Unmarshal(responses[0].Data, &items); err != nil || len(items) != 3 {
		t.Fatalf("search data = %s (%v); want three items", responses[0].Data, err)
	}
	if responses[0].ID != 1 || responses[3].ID != 4 {
		t.Fatalf("response ids not echoed: %d, %d", responses[0].ID, responses[3].ID)
	}
	if !strings.Contains(string(responses[3].Data), `"title":"ran hello"`) {
		t.Fatalf("run data = %s; want the action id echoed", responses[3].Data)
	}
}

func TestRecoveredPanicIsAFailureAndTheChildKeepsAnswering(t *testing.T) {
	_, responses := exchange(t, "acme.panics",
		protocol.Request{ID: 1, Method: protocol.MethodSearch, Query: "boom"},
		protocol.Request{ID: 2, Method: protocol.MethodSearch, Query: "fine"},
	)
	if len(responses) != 2 {
		t.Fatalf("got %d responses; want 2", len(responses))
	}
	failure := responses[0].Failure
	if failure == nil || failure.Code != api.CodeInternal || !strings.HasPrefix(failure.Reference, "ext-panic-") {
		t.Fatalf("failure = %+v; want internal with a panic reference", failure)
	}
	if responses[1].Failure != nil || !strings.Contains(string(responses[1].Data), "Still alive") {
		t.Fatalf("second response = %+v; want the child to keep answering", responses[1])
	}
}

func TestMissingMethodIsUnavailable(t *testing.T) {
	_, responses := exchange(t, "acme.slow", protocol.Request{ID: 1, Method: protocol.MethodRoot})
	if len(responses) != 1 || responses[0].Failure == nil || responses[0].Failure.Code != api.CodeUnavailable {
		t.Fatalf("response = %+v; want unavailable for a missing RootItems", responses)
	}
}

func TestUnknownMethodIsInvalidInput(t *testing.T) {
	_, responses := exchange(t, "acme.good", protocol.Request{ID: 1, Method: "explode"})
	if len(responses) != 1 || responses[0].Failure == nil || responses[0].Failure.Code != api.CodeInvalid {
		t.Fatalf("response = %+v; want invalid-input", responses)
	}
}

func TestSourceThatDoesNotCompileIsAReadyFailure(t *testing.T) {
	ready, responses := exchange(t, "acme.syntax")
	if ready.OK || ready.Failure == nil || !strings.Contains(ready.Failure.Message, "compile") {
		t.Fatalf("ready = %+v; want a compile failure", ready)
	}
	if len(responses) != 0 {
		t.Fatalf("a child that failed to load answered %d requests", len(responses))
	}
}

func TestMainIgnoresOrdinaryCommandLines(t *testing.T) {
	if _, handled := Main([]string{"nvm"}); handled {
		t.Fatal("Main claimed an ordinary app start")
	}
}
