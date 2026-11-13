package codemode

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ai/pi"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// piHost stands in for the pi sidecar process. It speaks the host protocol the engine speaks to the real script: it is
// ready at once, answers a prompt by calling run_program, and ends the reply when it has the tool's result. Nothing of
// the sidecar changes for run_program: the tool arrives in the prompt's tool list like every other host tool.
type piHost struct {
	mu       sync.Mutex
	code     string
	lines    chan []byte
	received []map[string]any
	tools    []string
}

func newPiHost(code string) *piHost {
	host := &piHost{code: code, lines: make(chan []byte, 16)}
	host.emit(map[string]any{"type": "ready"})
	return host
}

func (h *piHost) Start(platform.WorkerSpec) (platform.WorkerProcess, error) { return h, nil }
func (h *piHost) PID() int                                                  { return 0 }
func (h *piHost) Kill() error                                               { return nil }
func (h *piHost) Wait() (int, error)                                        { return 0, nil }
func (h *piHost) StderrTail() string                                        { return "" }

func (h *piHost) ReadLine() ([]byte, error) {
	line, ok := <-h.lines
	if !ok {
		return nil, io.EOF
	}
	return line, nil
}

func (h *piHost) emit(event map[string]any) {
	line, _ := json.Marshal(event)
	h.lines <- line
}

// WriteLine is the engine speaking to the host.
func (h *piHost) WriteLine(line []byte) error {
	var message map[string]any
	if err := json.Unmarshal(line, &message); err != nil {
		return err
	}
	h.mu.Lock()
	h.received = append(h.received, message)
	h.mu.Unlock()
	switch message["type"] {
	case "prompt":
		for _, tool := range message["tools"].([]any) {
			h.tools = append(h.tools, tool.(map[string]any)["name"].(string))
		}
		h.emit(map[string]any{"type": "step"})
		h.emit(map[string]any{"type": "tool_call", "callId": "c1", "name": ToolName, "arguments": map[string]string{"code": h.code}})
	case "tool_result":
		h.emit(map[string]any{"type": "delta", "text": "Done."})
		h.emit(map[string]any{"type": "done"})
	case "abort":
		h.emit(map[string]any{"type": "aborted"})
	}
	return nil
}

func (h *piHost) result() map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, message := range h.received {
		if message["type"] == "tool_result" {
			return message
		}
	}
	return nil
}

// piTurn runs one external turn through the real pi engine over the stand-in host.
func piTurn(t *testing.T, f *fixture, host *piHost) *agent.Turn {
	t.Helper()
	registry, err := agent.NewRegistry(Tool(f.runner))
	if err != nil {
		t.Fatal(err)
	}
	engine := pi.New(pi.Options{
		Worker:  host,
		Runtime: func() (pi.Runtime, error) { return pi.Runtime{Node: "node", Script: "host.mjs"}, nil },
		Access:  func() (pi.Access, error) { return pi.Access{BaseURL: "https://example.test", Token: "t"}, nil },
		Tools:   registry.All(),
		System:  func() string { return "system" },
	})
	t.Cleanup(engine.Close)
	history := []backend.Message{{Role: backend.RoleUser, Content: backend.TextContent("make a note")}}
	return agent.StartExternal(context.Background(), agent.Setup{Tools: registry, Clock: fake.NewClock()}, engine, history)
}

func TestPiToolCallReachesTheHostRunsTheProgramAndReturns(t *testing.T) {
	f := newFixture(t)
	host := newPiHost(`n, err := ctx.Notes().CreateWith("# From pi")
if err != nil { return nil, err }
return n.ID, nil`)
	events := drive(piTurn(t, f, host), func(agent.Event) { t.Fatal("a create asked the user") })
	if terminal := events[len(events)-1]; terminal.Kind != agent.EventDone {
		t.Fatalf("terminal = %s (%v)", terminal.Kind, terminal.Err)
	}
	if len(host.tools) != 1 || host.tools[0] != ToolName {
		t.Fatalf("the host was offered %v", host.tools)
	}
	result := host.result()
	if result == nil || result["status"] != "ran" || !strings.Contains(result["text"].(string), "created note") {
		t.Fatalf("the host got %+v", result)
	}
	if got := f.notes.List(); len(got) != 1 || got[0].Markdown != "# From pi" {
		t.Fatalf("notes = %+v", got)
	}
	starts := ofKind(events, agent.EventToolStart)
	if len(starts) != 2 || starts[1].Call.ParentID != "c1" {
		t.Fatalf("the transcript got %+v", starts)
	}
}

func TestPiToolCallAsksBeforeADeleteAndAnswersTheHostAfterwards(t *testing.T) {
	f := newFixture(t)
	note := f.note("# Old ideas")
	host := newPiHost(deleteOld)
	turn := piTurn(t, f, host)
	events := drive(turn, func(event agent.Event) {
		if _, found := f.notes.Get(note.ID); !found {
			t.Error("the note was deleted before the user answered")
		}
		turn.Decide(event.Call.ID, true)
	})
	if terminal := events[len(events)-1]; terminal.Kind != agent.EventDone {
		t.Fatalf("terminal = %s", terminal.Kind)
	}
	if _, found := f.notes.Get(note.ID); found {
		t.Fatal("the delete did not run")
	}
	if result := host.result(); result == nil || result["status"] != "ran" {
		t.Fatalf("the host got %+v", result)
	}
}

// Stop while the sidecar waits for the tool: the relay is busy inside the call, so the call must end with the turn.
func TestPiStopWhileTheQuestionIsUpEndsTheCallAndTheReply(t *testing.T) {
	f := newFixture(t)
	note := f.note("# Old ideas")
	host := newPiHost(deleteOld)
	turn := piTurn(t, f, host)
	events := drive(turn, func(agent.Event) { turn.Cancel() })
	if _, found := f.notes.Get(note.ID); !found {
		t.Fatal("stopping the chat deleted the note")
	}
	if terminal := events[len(events)-1]; terminal.Kind != agent.EventAborted {
		t.Fatalf("terminal = %s", terminal.Kind)
	}
	if result := host.result(); result == nil || result["status"] != "cancelled" {
		t.Fatalf("the host got %+v", result)
	}
	noChildren(t)
}
