package manager

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func toolsHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	if err := h.m.Install("acme.tools", fixtureContents(t, "acme.tools"), true); err != nil {
		t.Fatalf("Install: %v", err)
	}
	h.waitRunning("acme.tools")
	return h
}

// An enabled folder extension's tools are offered with the class the host enforces: its read is raised to a sensitive
// read, and its destructive tool stays destructive.
func TestAnEnabledExtensionsToolsAreOfferedWithTheEnforcedClass(t *testing.T) {
	h := toolsHarness(t)
	classes := map[string]api.AIToolClass{}
	for _, tool := range h.m.DeclaredAITools() {
		if tool.ExtensionID == "acme.tools" {
			classes[tool.Name] = tool.Class
		}
	}
	if classes["echo"] != api.AIToolSensitiveRead || classes["reset-count"] != api.AIToolDestructive || len(classes) != 2 {
		t.Fatalf("classes = %v", classes)
	}
}

func TestADisabledExtensionsToolsAreNotOfferedOrRun(t *testing.T) {
	h := toolsHarness(t)
	if err := h.m.Disable("acme.tools"); err != nil {
		t.Fatal(err)
	}
	if tools := h.m.DeclaredAITools(); len(tools) != 0 {
		t.Fatalf("a disabled extension offers %+v", tools)
	}
	host := NewAIToolHost(func() AIToolSource { return h.m })
	_, err := host.Call(context.Background(), "acme.tools", "echo", json.RawMessage(`{"text":"hi"}`), false)
	if !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("err = %v; want the tool refused as unknown", err)
	}
}

// A call reaches the extension's child with its arguments, and its describe reads the extension's own storage.
func TestAToolCallRunsInTheExtensionsChild(t *testing.T) {
	h := toolsHarness(t)
	host := NewAIToolHost(func() AIToolSource { return h.m })
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	result, err := host.Call(ctx, "acme.tools", "echo", json.RawMessage(`{"text":"hi"}`), false)
	if err != nil || string(result) != `{"text":"hi"}` {
		t.Fatalf("result = %s, %v", result, err)
	}
	plan, err := host.Plan(ctx, "acme.tools", "reset-count", nil)
	if err != nil || plan.Question() != "Tools fixture: Reset the count from 0 to 0" {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
}

func TestToolArgumentsMustFitTheInputSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"},"limit":{"type":"integer"}},"required":["id"],"additionalProperties":false}`)
	for name, args := range map[string]string{
		"not an object": `[1]`, "missing": `{}`, "wrong type": `{"id":1}`, "fraction": `{"id":"a","limit":1.5}`, "unknown": `{"id":"a","x":1}`,
	} {
		if _, err := checkAIToolArgs(schema, json.RawMessage(args)); !errors.Is(err, api.ErrInvalidInput) {
			t.Errorf("%s: err = %v; want a refusal", name, err)
		}
	}
	if args, err := checkAIToolArgs(nil, nil); err != nil || string(args) != "{}" {
		t.Fatalf("no arguments = %s, %v; want the empty object", args, err)
	}
	if _, err := checkAIToolArgs(schema, json.RawMessage(`{"id":"a","limit":3}`)); err != nil {
		t.Fatalf("fitting arguments refused: %v", err)
	}
}
