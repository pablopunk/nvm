package spike

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"testing"
)

func drain(events <-chan Event) []Event {
	var collected []Event
	for event := range events {
		collected = append(collected, event)
	}
	return collected
}

// summarize renders each event as one golden line, so a scenario reads as its exact sequence.
func summarize(events []Event) []string {
	lines := make([]string, 0, len(events))
	for _, event := range events {
		switch event.Kind {
		case EventDelta:
			lines = append(lines, "delta:"+event.Text)
		case EventToolStart:
			lines = append(lines, "tool_start:"+event.Tool)
		case EventToolEnd:
			lines = append(lines, fmt.Sprintf("tool_end:%s:%s", event.Tool, outcomeWord(event.IsError)))
		case EventError:
			lines = append(lines, "error:"+event.Code)
		default:
			lines = append(lines, string(event.Kind))
		}
	}
	return lines
}

func outcomeWord(isError bool) string {
	if isError {
		return "error"
	}
	return "ok"
}

func textChunks(parts ...string) []ModelChunk {
	chunks := make([]ModelChunk, 0, len(parts))
	for _, part := range parts {
		chunks = append(chunks, ModelChunk{Text: part})
	}
	return chunks
}

// callChunks streams one tool call in pieces: the first piece carries the id and name, and the arguments follow.
func callChunks(index int, id, name string, argumentPieces ...string) []ModelChunk {
	chunks := make([]ModelChunk, 0, len(argumentPieces))
	for position, piece := range argumentPieces {
		delta := ToolCallDelta{Index: index, ArgumentsDelta: piece}
		if position == 0 {
			delta.ID = id
			delta.Name = name
		}
		chunks = append(chunks, ModelChunk{ToolDelta: &delta})
	}
	return chunks
}

func concatChunks(parts ...[]ModelChunk) []ModelChunk {
	var chunks []ModelChunk
	for _, part := range parts {
		chunks = append(chunks, part...)
	}
	return chunks
}

func newTestRegistry(t *testing.T, tools ...Tool) *Registry {
	t.Helper()
	registry, err := NewRegistry(tools...)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return registry
}

func constantTool(name, output string) Tool {
	return Tool{
		Spec: ToolSpec{Name: name, Description: "test tool " + name},
		Run: func(context.Context, json.RawMessage) (string, error) {
			return output, nil
		},
	}
}

// countingTool records each run in runs. The loop runs tools on its own goroutine and the test reads runs only after
// the channel closes, so the count needs no lock.
func countingTool(name, output string, runs *int) Tool {
	return Tool{
		Spec: ToolSpec{Name: name, Description: "test tool " + name},
		Run: func(context.Context, json.RawMessage) (string, error) {
			*runs++
			return output, nil
		},
	}
}

func failingTool(name string, failure error) Tool {
	return Tool{
		Spec: ToolSpec{Name: name, Description: "test tool " + name},
		Run: func(context.Context, json.RawMessage) (string, error) {
			return "", failure
		},
	}
}

// waitForGoroutines polls the scheduler, without a timer, until the goroutine count returns to baseline.
func waitForGoroutines(t *testing.T, baseline int) {
	t.Helper()
	for range 10000 {
		if runtime.NumGoroutine() <= baseline {
			return
		}
		runtime.Gosched()
	}
	t.Fatalf("goroutines did not return to %d; now %d", baseline, runtime.NumGoroutine())
}

func countTerminal(events []Event) int {
	count := 0
	for _, event := range events {
		switch event.Kind {
		case EventDone, EventError, EventAborted:
			count++
		}
	}
	return count
}

var errConnectionRefused = errors.New("connection refused")
