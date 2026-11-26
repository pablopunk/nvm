package codemode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

var schemaWithID = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"],"additionalProperties":false}`)

// The enabled extensions' tools, as their manifests declare them. Weather is a folder extension, so its read is raised
// to a sensitive read; Snippets is a built-in, so its search stays a read.
var (
	snippetsManifest = api.Manifest{Identity: api.Identity{ID: "nevermind.snippets", Title: "Snippets"}, AITools: []api.AITool{
		{Name: "search-snippets", Description: "Searches snippets.", Class: api.AIToolRead,
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`)},
		{Name: "delete-snippet", Description: "Deletes a snippet.", Class: api.AIToolDestructive, InputSchema: schemaWithID},
	}}
	weatherManifest = api.Manifest{Identity: api.Identity{ID: "acme.weather", Title: "Weather"}, AITools: []api.AITool{
		{Name: "forecast", Description: "Today's forecast.", Class: api.AIToolRead},
	}}
)

// fakeToolSource is the extension manager's side of the tools, under the host's real tool host: the declared tools,
// the snippets the Snippets tools describe and change, and every call that ran with whether the user had approved it.
type fakeToolSource struct {
	mu       sync.Mutex
	snippets map[string]string
	ran      []string
	approved []bool
}

func (s *fakeToolSource) DeclaredAITools() []api.AIToolRecord {
	return append(toolRecordsOf(snippetsManifest), toolRecordsOf(weatherManifest)...)
}

func toolRecordsOf(manifest api.Manifest) []api.AIToolRecord {
	var records []api.AIToolRecord
	for _, tool := range manifest.AITools {
		records = append(records, api.AIToolRecord{ExtensionID: manifest.ID, ExtensionTitle: manifest.Title, Name: tool.Name,
			Description: tool.Description, InputSchema: tool.InputSchema, Class: manifest.EnforcedAIToolClass(tool)})
	}
	return records
}

func (s *fakeToolSource) DescribeAITool(_ context.Context, _ string, call api.AIToolCall) (string, error) {
	if call.Name != "delete-snippet" {
		return "", nil
	}
	name, found := s.snippet(call)
	if !found {
		return "", fmt.Errorf("this snippet no longer exists: %w", api.ErrInvalidInput)
	}
	return fmt.Sprintf("Delete the snippet “%s” (keyword “sig”)", name), nil
}

func (s *fakeToolSource) CallAITool(ctx context.Context, id string, call api.AIToolCall) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ran = append(s.ran, id+"/"+call.Name)
	s.approved = append(s.approved, protocol.UserApproved(ctx))
	switch call.Name {
	case "delete-snippet":
		var args struct{ ID string }
		_ = json.Unmarshal(call.Args, &args)
		delete(s.snippets, args.ID)
		return json.RawMessage("null"), nil
	case "forecast":
		return json.RawMessage(`{"sky":"sunny"}`), nil
	}
	return json.RawMessage(`[{"id":"snip-1","name":"Signature"}]`), nil
}

func (s *fakeToolSource) snippet(call api.AIToolCall) (string, bool) {
	var args struct{ ID string }
	_ = json.Unmarshal(call.Args, &args)
	s.mu.Lock()
	defer s.mu.Unlock()
	name, found := s.snippets[args.ID]
	return name, found
}

func (s *fakeToolSource) calls() ([]string, []bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.ran...), append([]bool(nil), s.approved...)
}

func toolsFixture(t *testing.T) (*fixture, *fakeToolSource) {
	source := &fakeToolSource{snippets: map[string]string{"snip-1": "Signature"}}
	host := manager.NewAIToolHost(func() manager.AIToolSource { return source })
	return newFixture(t, func(opts *Options) { opts.Tools = host }), source
}

const deleteSignature = `return ctx.Tools().Call("nevermind.snippets", "delete-snippet", map[string]any{"id": "snip-1"})`

func TestToolOperationsTakeTheirClassFromTheTool(t *testing.T) {
	rules := toolRules()
	if len(rules) != 2 || rules[key{protocol.ServiceTools, protocol.OpList}].class != ClassRead {
		t.Fatalf("rules = %+v", rules)
	}
	call := rules[key{protocol.ServiceTools, protocol.OpCall}]
	if call.classify == nil || !call.class.asksEachTime() {
		t.Fatalf("a call must take the tool's class, and fall back to one that asks every time: %+v", call)
	}
}

// Every class an extension may declare is a class the gate knows, with the same name.
func TestEveryToolClassIsAGateClass(t *testing.T) {
	known := map[Class]bool{ClassRead: true, ClassWrite: true, ClassSensitiveRead: true, ClassDestructive: true, ClassExternal: true, ClassSetting: true}
	for _, class := range []api.AIToolClass{api.AIToolRead, api.AIToolWrite, api.AIToolSensitiveRead, api.AIToolDestructive, api.AIToolExternal, api.AIToolSetting} {
		if !known[Class(class)] {
			t.Errorf("tool class %q is not a gate class", class)
		}
	}
}

// A destructive tool asks with the extension's title and its own description of the call, and runs once allowed, with
// the approval passed to the extension so its own services do not ask again.
func TestADestructiveToolAsksWithItsDescriptionAndRunsWhenAllowed(t *testing.T) {
	f, source := toolsFixture(t)
	f.steps.decide = allow
	result := f.run(deleteSignature)
	ran, approved := source.calls()
	if result.Failed || len(ran) != 1 || !approved[0] {
		t.Fatalf("result = %+v, ran %v approved %v", result, ran, approved)
	}
	row := lastRow(t, f, "Using an extension tool")
	t.Logf("question: %s\nmodel reads: %s", row.detail, result.Text)
	if !row.asked || row.detail != "Snippets: Delete the snippet “Signature” (keyword “sig”)" {
		t.Fatalf("row = %+v", row)
	}
	contains(t, result.Text, "used “delete-snippet” of Snippets: delete the snippet “Signature” (keyword “sig”) (the user allowed it)")
}

func TestADeclinedToolCallRunsNothing(t *testing.T) {
	f, source := toolsFixture(t)
	f.steps.decide = decline
	result := f.run(deleteSignature)
	if ran, _ := source.calls(); !result.Failed || len(ran) != 0 {
		t.Fatalf("result = %+v, ran %v", result, ran)
	}
	contains(t, result.Text, "snippets: Delete the snippet “Signature” (keyword “sig”): the user declined")
}

// A built-in's read runs without asking, and without an approval its services could lean on.
func TestAReadToolRunsWithoutAsking(t *testing.T) {
	f, source := toolsFixture(t)
	result := f.run(`return ctx.Tools().Call("nevermind.snippets", "search-snippets", map[string]any{"query": "sig"})`)
	ran, approved := source.calls()
	if result.Failed || len(ran) != 1 || approved[0] {
		t.Fatalf("result = %+v, ran %v approved %v", result, ran, approved)
	}
	if row := lastRow(t, f, "Using an extension tool"); row.asked {
		t.Fatalf("a read asked: %+v", row)
	}
	contains(t, result.Text, `"name":"Signature"`)
}

// A folder extension's read is raised to a sensitive read: the user is asked once in a run for that extension, and the
// answer is no approval for anything that removes data.
func TestAFolderExtensionsReadAsksOncePerRun(t *testing.T) {
	f, source := toolsFixture(t)
	f.steps.decide = allow
	result := f.run(`a, err := ctx.Tools().Call("acme.weather", "forecast", nil)
if err != nil { return nil, err }
b, err := ctx.Tools().Call("acme.weather", "forecast", nil)
return []any{a, b}, err`)
	ran, approved := source.calls()
	if result.Failed || len(ran) != 2 || approved[0] || approved[1] {
		t.Fatalf("result = %+v, ran %v approved %v", result, ran, approved)
	}
	asked := 0
	for _, row := range f.steps.snapshot() {
		if row.asked {
			asked++
		}
	}
	if asked != 1 {
		t.Fatalf("asked %d times; want once per extension and run", asked)
	}
}

// A call the host refuses while planning it, such as an unknown tool, arguments that do not fit, or a snippet the
// extension no longer has, fails with the reason and is never put to the user.
func TestAToolCallTheHostRefusesFailsWithoutAsking(t *testing.T) {
	cases := map[string]struct{ body, reason string }{
		"unknown tool":     {`return ctx.Tools().Call("nevermind.snippets", "rename-snippet", nil)`, "has the tool"},
		"missing argument": {`return ctx.Tools().Call("nevermind.snippets", "delete-snippet", nil)`, `"id" is required`},
		"wrong type":       {`return ctx.Tools().Call("nevermind.snippets", "delete-snippet", map[string]any{"id": 3})`, "must be of type string"},
		"unknown argument": {`return ctx.Tools().Call("nevermind.snippets", "delete-snippet", map[string]any{"id": "snip-1", "all": true})`, "takes no argument"},
		"gone snippet":     {`return ctx.Tools().Call("nevermind.snippets", "delete-snippet", map[string]any{"id": "snip-9"})`, "no longer exists"},
		"disabled":         {`return ctx.Tools().Call("acme.off", "forecast", nil)`, "no enabled extension"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f, source := toolsFixture(t)
			f.steps.decide = allow
			result := f.run(c.body)
			t.Logf("model reads: %s", result.Text)
			if !result.Failed || !strings.Contains(result.Text, c.reason) {
				t.Fatalf("result = %+v", result)
			}
			if row := lastRow(t, f, "Using an extension tool"); row.asked {
				t.Fatalf("a refused call was put to the user: %+v", row)
			}
			if ran, _ := source.calls(); len(ran) != 0 {
				t.Fatalf("a refused call ran: %v", ran)
			}
		})
	}
}

func TestListingToolsDoesNotAsk(t *testing.T) {
	f, _ := toolsFixture(t)
	result := f.run(`return len(ctx.Tools().List("snippet")), nil`)
	if result.Failed || !strings.Contains(result.Text, "Result: 2") {
		t.Fatalf("result = %+v", result)
	}
	if row := lastRow(t, f, "Listing extension tools"); row.asked {
		t.Fatalf("a read asked: %+v", row)
	}
}
