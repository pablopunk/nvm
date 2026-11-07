package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// watchdog bounds a wait for the loop. Waits end as soon as the event arrives; this only fails a broken loop.
const watchdog = 10 * time.Second

// echoSchema requires one string argument named text.
var echoSchema = json.RawMessage(`{"type":"object","properties":{"text":{"type":"string","maxLength":200}},"required":["text"],"additionalProperties":false}`)

// echoTool returns its text argument and counts its runs.
func echoTool(runs *atomic.Int32) Tool {
	return Tool{
		Name: "echo", Label: "Echoing", Schema: echoSchema,
		Summarize: func(args json.RawMessage) string {
			var in struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(args, &in)
			return in.Text
		},
		Run: func(_ context.Context, args json.RawMessage) (Result, error) {
			runs.Add(1)
			var in struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return Result{}, err
			}
			return Result{Text: "echo: " + in.Text, Outcome: "Echoed"}, nil
		},
	}
}

// registry builds a registry or fails the test.
func registry(t *testing.T, tools ...Tool) *Registry {
	t.Helper()
	reg, err := NewRegistry(tools...)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return reg
}

// setup returns a turn setup over model and reg on a fake clock.
func setup(model Model, reg *Registry, clock *fake.Clock) Setup {
	return Setup{Model: model, Tools: reg, Clock: clock}
}

// call is one tool call the model made, with its arguments in one piece.
func call(index int, id, name, arguments string) backend.ToolCallDelta {
	return backend.ToolCallDelta{Index: index, ID: id, Name: name, ArgumentsDelta: arguments}
}

// callChunks is a model turn that asks for calls.
func callChunks(deltas ...backend.ToolCallDelta) ScriptedTurn {
	chunks := make([]ModelChunk, 0, len(deltas))
	for i := range deltas {
		chunks = append(chunks, ModelChunk{ToolDelta: &deltas[i]})
	}
	return ScriptedTurn{Chunks: chunks}
}

// textTurn is a model turn that answers with text.
func textTurn(pieces ...string) ScriptedTurn {
	chunks := make([]ModelChunk, 0, len(pieces))
	for _, piece := range pieces {
		chunks = append(chunks, ModelChunk{Text: piece})
	}
	return ScriptedTurn{Chunks: chunks}
}

// drain reads a turn's events until the channel closes, failing the test if it never does.
func drain(t *testing.T, turn *Turn) []Event {
	t.Helper()
	var events []Event
	deadline := time.After(watchdog)
	for {
		select {
		case event, ok := <-turn.Events():
			if !ok {
				return events
			}
			events = append(events, event)
		case <-deadline:
			t.Fatalf("the turn never ended; events so far: %s", kinds(events))
		}
	}
}

// kinds names the events of a turn in order, for sequence assertions. The drafting hints are left out: they are shown
// while a call streams and are checked on their own.
func kinds(events []Event) string {
	names := make([]string, 0, len(events))
	for _, event := range events {
		if event.Kind == EventCallDrafting {
			continue
		}
		names = append(names, string(event.Kind))
	}
	return strings.Join(names, " ")
}

// terminal returns the last event of a turn.
func terminal(t *testing.T, events []Event) Event {
	t.Helper()
	if len(events) == 0 {
		t.Fatal("the turn sent no events")
	}
	return events[len(events)-1]
}

// user is the history of a turn that asks text.
func user(text string) []backend.Message {
	return []backend.Message{{Role: backend.RoleUser, Content: backend.TextContent(text)}}
}

// eventsOf returns the events of one kind.
func eventsOf(events []Event, kind EventKind) []Event {
	var out []Event
	for _, event := range events {
		if event.Kind == kind {
			out = append(out, event)
		}
	}
	return out
}
