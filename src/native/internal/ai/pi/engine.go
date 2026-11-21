package pi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// maxLineBytes bounds one line the host writes: a reply delta or a tool call.
const maxLineBytes = 8 << 20

// abortGrace is how long a stopped reply waits for the host to say it stopped before the process is killed.
const abortGrace = 5 * time.Second

// freeSpendCapCode is the reason the backend gives for pausing the free pool; the chat shows its notice for it. The host
// reports it when the route is blocked, and paymentRequired is the status the backend refuses such a request with.
const (
	freeSpendCapCode = "free_spend_cap_reached"
	paymentRequired  = 402
)

// maxHistoryRunes bounds the earlier conversation sent to a host that has no context.
const maxHistoryRunes = 40_000

// Runtime is where the host's files are: the Node binary and the script it runs.
type Runtime struct {
	Node   string
	Script string
}

// Access is what the host needs to call the model proxy as the signed-in user.
type Access struct {
	BaseURL string
	Token   string
	Headers map[string]string
}

// Options is what an Engine runs on. Worker, Runtime and Access are required.
type Options struct {
	Worker platform.Worker
	// Runtime finds the bundled Node and the host script; it fails when this build does not ship them.
	Runtime func() (Runtime, error)
	// Access returns the origin, token and headers of the backend; it fails when the user is not signed in.
	Access func() (Access, error)
	// Tools are the app's tools the model may call, in the order the model sees them.
	Tools []agent.Tool
	// System is the instruction the chat starts with.
	System func() string
	// Role is the model tier the chat uses, sent to the backend as the model role.
	Role         string
	WorkspaceDir string
	AgentDir     string
}

// Engine runs one chat's replies on a pi host process. It is safe for one reply at a time, as a chat is.
type Engine struct {
	opts Options

	mu       sync.Mutex
	proc     platform.WorkerProcess
	lines    chan []byte
	hasState bool
	// held is the characters the host's session already holds besides the instruction: earlier prompts and replies.
	held   int
	nextID int
}

// New returns an engine that starts its host on the first reply.
func New(opts Options) *Engine {
	return &Engine{opts: opts}
}

// Close stops the host. The next reply starts a new one, without the chat's earlier context.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopLocked()
}

func (e *Engine) stopLocked() {
	if e.proc != nil {
		_ = e.proc.Kill()
		e.proc, e.lines, e.hasState, e.held = nil, nil, false, 0
	}
}

// Run implements agent.Engine.
func (e *Engine) Run(ctx context.Context, run agent.ExternalRun, history []backend.Message) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	prompt, err := e.prompt(history)
	if err != nil {
		return err
	}
	if err := e.ensureLocked(ctx); err != nil {
		return err
	}
	if err := e.send(prompt); err != nil {
		e.stopLocked()
		return fmt.Errorf("the assistant could not start: %w", err)
	}
	e.hasState = true
	e.held = boundChars(e.held + len(prompt.History) + len(prompt.Text))
	return e.relay(ctx, run)
}

// ensureLocked starts the host when it is not running and waits until it says it is ready.
func (e *Engine) ensureLocked(ctx context.Context) error {
	if e.proc != nil {
		return nil
	}
	runtime, err := e.opts.Runtime()
	if err != nil {
		return err
	}
	proc, err := e.opts.Worker.Start(platform.WorkerSpec{Binary: runtime.Node, Args: []string{runtime.Script}, MaxLineBytes: maxLineBytes})
	if err != nil {
		return fmt.Errorf("the assistant could not start: %w", err)
	}
	lines := make(chan []byte, 64)
	go pump(proc, lines)
	e.proc, e.lines = proc, lines
	select {
	case line, ok := <-lines:
		var event hostEvent
		if !ok || json.Unmarshal(line, &event) != nil || event.Type != "ready" {
			e.stopLocked()
			return errors.New("the assistant did not start")
		}
		return nil
	case <-ctx.Done():
		e.stopLocked()
		return ctx.Err()
	case <-time.After(30 * time.Second):
		e.stopLocked()
		return errors.New("the assistant took too long to start")
	}
}

// pump copies the host's lines onto a channel and closes it when the host exits.
func pump(proc platform.WorkerProcess, lines chan<- []byte) {
	defer close(lines)
	for {
		line, err := proc.ReadLine()
		if err != nil {
			return
		}
		lines <- line
	}
}

func (e *Engine) send(message any) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return e.proc.WriteLine(data)
}

// prompt builds the message that starts a reply to the last message of history.
func (e *Engine) prompt(history []backend.Message) (promptMessage, error) {
	access, err := e.opts.Access()
	if err != nil {
		return promptMessage{}, err
	}
	if len(history) == 0 || history[len(history)-1].Role != backend.RoleUser {
		return promptMessage{}, errors.New("a reply needs a message from the user")
	}
	text, images := userContent(history[len(history)-1])
	message := e.message("prompt", fmt.Sprintf("reply-%d", e.nextIDLocked()), access)
	message.Text, message.Images = text, images
	if !e.hasState {
		message.History = transcript(history[:len(history)-1])
	}
	message.Chars = e.requestChars(message)
	return message, nil
}

func (e *Engine) nextIDLocked() int {
	e.nextID++
	return e.nextID
}

// message is a request to the host with everything but the user's text: the access, the instruction and the tools.
func (e *Engine) message(kind, id string, access Access) promptMessage {
	return promptMessage{
		Type: kind, ID: id, BaseURL: access.BaseURL, Token: access.Token, Headers: access.Headers,
		Role: e.opts.Role, System: e.system(), Tools: definitions(e.opts.Tools),
		WorkspaceDir: e.opts.WorkspaceDir, AgentDir: e.opts.AgentDir,
	}
}

func (e *Engine) system() string {
	if e.opts.System == nil {
		return ""
	}
	return e.opts.System()
}

func definitions(tools []agent.Tool) []toolDefinition {
	out := make([]toolDefinition, 0, len(tools))
	for _, tool := range tools {
		out = append(out, toolDefinition{Name: tool.Name, Label: tool.Label, Description: tool.Description, Parameters: tool.Schema})
	}
	return out
}

// relay reads the host's events until the reply ends, reporting them through run and answering its tool calls.
func (e *Engine) relay(ctx context.Context, run agent.ExternalRun) error {
	step := 0
	aborted := false
	var killAt <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			if !aborted {
				aborted = true
				_ = e.send(abortMessage{Type: "abort"})
				killAt = time.After(abortGrace)
			}
			ctx = context.Background() // keep draining until the host says it stopped
		case <-killAt:
			e.stopLocked()
			return context.Canceled
		case line, ok := <-e.lines:
			if !ok {
				e.stopLocked()
				if aborted {
					return context.Canceled
				}
				return errors.New("the assistant stopped unexpectedly")
			}
			finished, err := e.handle(line, run, &step)
			if finished {
				if aborted && err == nil {
					return context.Canceled
				}
				return err
			}
		}
	}
}

// handle acts on one event. It reports finished when the reply is over, with the failure that ended it, if any.
func (e *Engine) handle(line []byte, run agent.ExternalRun, step *int) (bool, error) {
	var event hostEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return false, nil
	}
	switch event.Type {
	case "delta":
		e.held = boundChars(e.held + len(event.Text))
		run.Delta(event.Text)
	case "reasoning":
		run.Reasoning(event.Text)
	case "step":
		*step++
		run.Step(*step)
	case "native_tool":
		call := nativeCall(event)
		if event.Phase == "start" {
			run.NativeStart(call)
		} else {
			run.NativeEnd(call, event.IsError)
		}
	case "tool_call":
		e.answer(run, event)
	case "done":
		if event.Usage != nil {
			run.AddUsage(agent.Usage{InputTokens: event.Usage.InputTokens, OutputTokens: event.Usage.OutputTokens})
		}
		return true, nil
	case "aborted":
		return true, context.Canceled
	case "error":
		return true, hostError(event)
	}
	return false, nil
}

// answer has the agent package judge and run a tool call of the host, and sends the host what the model reads.
func (e *Engine) answer(run agent.ExternalRun, event hostEvent) {
	call := backend.ToolCall{ID: event.CallID, Type: "function", Function: backend.ToolFunction{Name: event.Name, Arguments: string(event.Arguments)}}
	text, status := run.RunCall(call)
	e.held = boundChars(e.held + len(text))
	_ = e.send(toolResultMessage{Type: "tool_result", CallID: event.CallID, Text: text, Status: string(status)})
}

// nativeCall describes a tool the host runs itself, with the words the transcript shows.
func nativeCall(event hostEvent) agent.CallInfo {
	return agent.CallInfo{ID: event.CallID, Name: event.Name, Label: nativeLabel(event.Name), Input: summarize(event.Args)}
}

func nativeLabel(name string) string {
	switch name {
	case "web_search":
		return "Searching the web"
	case "fetch_content":
		return "Reading a web page"
	}
	return name
}

// summarize is the one line of a call's arguments: its query or address.
func summarize(args json.RawMessage) string {
	var fields map[string]any
	if json.Unmarshal(args, &fields) != nil {
		return ""
	}
	for _, key := range []string{"query", "queries", "url", "urls"} {
		switch value := fields[key].(type) {
		case string:
			return value
		case []any:
			parts := make([]string, 0, len(value))
			for _, item := range value {
				if text, ok := item.(string); ok {
					parts = append(parts, text)
				}
			}
			return strings.Join(parts, ", ")
		}
	}
	return ""
}

// hostError is the failure the host reported. A sign-in failure keeps its own type, so the chat shows the sign-in notice.
func hostError(event hostEvent) error {
	if event.Code == "auth_required" {
		return &backend.Error{Kind: backend.KindUnauthorized, Message: event.Message}
	}
	if event.Code == freeSpendCapCode {
		return &backend.Error{Kind: backend.KindRejected, Status: paymentRequired, Code: freeSpendCapCode, Message: "the route is paused by the free spend cap"}
	}
	if event.Message == "" {
		return errors.New("the assistant failed")
	}
	return errors.New(event.Message)
}
