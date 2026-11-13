package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// TestValidateArgumentsTable checks the schema subset the tools rely on.
func TestValidateArgumentsTable(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{
		"path":{"type":"string","minLength":1},
		"limit":{"type":"integer","minimum":1,"maximum":20},
		"ratio":{"type":"number"},
		"force":{"type":"boolean"},
		"tags":{"type":"array","items":{"type":"string"},"maxItems":2},
		"mode":{"type":"string","enum":["fast","smart"]}
	},"required":["path"],"additionalProperties":false}`)
	tests := []struct {
		name string
		args string
		ok   bool
	}{
		{"minimal", `{"path":"a"}`, true},
		{"all fields", `{"path":"a","limit":5,"ratio":0.5,"force":true,"tags":["x"],"mode":"fast"}`, true},
		{"integer as float text", `{"path":"a","limit":5.0}`, true},
		{"fraction is not an integer", `{"path":"a","limit":1.5}`, false},
		{"below minimum", `{"path":"a","limit":0}`, false},
		{"above maximum", `{"path":"a","limit":21}`, false},
		{"empty path", `{"path":""}`, false},
		{"missing required", `{"limit":2}`, false},
		{"wrong boolean", `{"path":"a","force":"yes"}`, false},
		{"too many tags", `{"path":"a","tags":["x","y","z"]}`, false},
		{"tag of wrong type", `{"path":"a","tags":[1]}`, false},
		{"enum miss", `{"path":"a","mode":"turbo"}`, false},
		{"unknown field", `{"path":"a","extra":true}`, false},
		{"trailing value", `{"path":"a"} {}`, false},
		{"array instead of object", `["a"]`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArguments(schema, json.RawMessage(tt.args))
			if (err == nil) != tt.ok {
				t.Fatalf("ValidateArguments(%s) = %v, want ok=%v", tt.args, err, tt.ok)
			}
			if err != nil && !errors.Is(err, ErrArguments) {
				t.Fatalf("error %v does not wrap ErrArguments", err)
			}
		})
	}
}

// TestEmptySchemaAcceptsAnything keeps keyless tools working.
func TestEmptySchemaAcceptsAnything(t *testing.T) {
	if err := ValidateArguments(nil, json.RawMessage(`{"x":1}`)); err != nil {
		t.Fatalf("empty schema rejected arguments: %v", err)
	}
}

// TestRegistryRejectsDuplicatesAndIncompleteTools checks construction errors.
func TestRegistryRejectsDuplicatesAndIncompleteTools(t *testing.T) {
	run := func(context.Context, json.RawMessage) (Result, error) { return Result{}, nil }
	if _, err := NewRegistry(Tool{Name: "a", Run: run}, Tool{Name: "a", Run: run}); err == nil {
		t.Fatal("duplicate names were accepted")
	}
	if _, err := NewRegistry(Tool{Name: "", Run: run}); err == nil {
		t.Fatal("a tool without a name was accepted")
	}
	if _, err := NewRegistry(Tool{Name: "b"}); err == nil {
		t.Fatal("a tool without a run function was accepted")
	}
}

// TestWireKeepsRegistrationOrder checks the tools array the request sends.
func TestWireKeepsRegistrationOrder(t *testing.T) {
	run := func(context.Context, json.RawMessage) (Result, error) { return Result{}, nil }
	reg := registry(t, Tool{Name: "first", Description: "one", Schema: json.RawMessage(`{"type":"object"}`), Run: run},
		Tool{Name: "second", Schema: json.RawMessage(`{"type":"object"}`), Run: run})
	wire, err := reg.Wire()
	if err != nil {
		t.Fatal(err)
	}
	var decoded []struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 2 || decoded[0].Function.Name != "first" || decoded[1].Type != "function" {
		t.Fatalf("wire = %s", wire)
	}
	empty, _ := (*Registry)(nil).Wire()
	if empty != nil {
		t.Fatal("a nil registry offered tools")
	}
}
