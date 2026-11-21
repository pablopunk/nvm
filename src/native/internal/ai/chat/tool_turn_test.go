package chat

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// toolSchema is the schema of the test tools: one required text argument.
var toolSchema = json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"],"additionalProperties":false}`)

// echoTool returns its text argument and counts its runs.
func echoTool(runs *atomic.Int32) agent.Tool {
	return agent.Tool{
		Name: "echo", Label: "Echoing", Schema: toolSchema,
		Run: func(_ context.Context, args json.RawMessage) (agent.Result, error) {
			runs.Add(1)
			var in struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(args, &in)
			return agent.Result{Text: "echo: " + in.Text, Outcome: "Echoed"}, nil
		},
	}
}

// deleteTool asks the user before it runs, and counts its runs.
func deleteTool(runs *atomic.Int32) agent.Tool {
	tool := echoTool(runs)
	tool.Name, tool.Label, tool.Confirm = "delete", "Deleting", true
	return tool
}

// blockingTool runs until its turn is cancelled. started closes once it runs.
func blockingTool(started chan struct{}) agent.Tool {
	return agent.Tool{
		Name: "wait", Label: "Waiting", Schema: json.RawMessage(`{"type":"object"}`),
		Run: func(ctx context.Context, _ json.RawMessage) (agent.Result, error) {
			close(started)
			<-ctx.Done()
			return agent.Result{}, ctx.Err()
		},
	}
}

// toolsOf builds the toolset of a test from tools.
func toolsOf(t *testing.T, tools ...agent.Tool) Toolset {
	t.Helper()
	registry, err := agent.NewRegistry(tools...)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return Toolset{Registry: registry, Policy: agent.AllowAll{}}
}

// awaitState runs queued work until the transcript satisfies done, failing the test if it never does.
func (f *fixture) awaitState(done func(messages []Message) bool) {
	f.t.Helper()
	deadline := time.After(watchdog)
	for {
		f.q.drain()
		if chat, ok := f.svc.Chat(f.lastChat()); ok && done(chat.Messages) {
			return
		}
		select {
		case <-f.q.notify:
		case <-deadline:
			f.t.Fatal("the transcript never reached the state the test waits for")
		}
	}
}

// lastChat is the id of the chat the fixture was last asked about; tests keep one chat per fixture.
func (f *fixture) lastChat() string {
	chats := f.svc.Chats()
	if len(chats) == 0 {
		return ""
	}
	return chats[0].ID
}

// toolRows returns the tool entries of a transcript, in order.
func toolRows(messages []Message) []Message {
	var rows []Message
	for _, message := range messages {
		if message.Author == AuthorTool {
			rows = append(rows, message)
		}
	}
	return rows
}

// hasToolWith reports whether a tool entry in messages has status.
func hasToolWith(messages []Message, status Status) bool {
	for _, row := range toolRows(messages) {
		if row.Status == status {
			return true
		}
	}
	return false
}

// noticeKinds lists the notice kinds of a transcript.
func noticeKinds(messages []Message) []NoticeKind {
	var kinds []NoticeKind
	for _, message := range messages {
		if message.Author == AuthorNotice {
			kinds = append(kinds, message.Notice.Kind)
		}
	}
	return kinds
}

// TestToolTurnRecordsTheTranscriptAndAnswers covers the golden path: text, a call, its result, and the final answer.
func TestToolTurnRecordsTheTranscriptAndAnswers(t *testing.T) {
	var runs atomic.Int32
	backend := fixtures.NewScriptedBackend(
		fixtures.ToolCallReply("Let me check. ", fixtures.ToolCall{ID: "call_1", Name: "echo", Arguments: []string{`{"text":`, `"ping"}`}}),
		fixtures.TextReply("It says ping."),
	)
	f := newScriptedFixture(t, toolsOf(t, echoTool(&runs)), backend)
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, "echo ping"); err != nil {
		t.Fatal(err)
	}
	f.awaitTurn(chat.ID)
	messages := f.messages(chat.ID)
	if got := kindsOf(messages); got != "user assistant tool assistant" {
		t.Fatalf("transcript = %q", got)
	}
	reply := messages[1]
	if reply.Text != "Let me check. " || reply.Status != StatusComplete || len(reply.Calls) != 1 || reply.Calls[0].ID != "call_1" {
		t.Fatalf("the reply with the call = %+v", reply)
	}
	row := messages[2]
	if row.Status != StatusComplete || row.Text != "echo: ping" || row.Tool.Outcome != "Echoed" || row.Tool.Label != "Echoing" || row.Tool.Input != "" {
		t.Fatalf("the tool row = %+v, %+v", row, row.Tool)
	}
	if last, _ := LastReply(f.chatOf(chat.ID)); last != "It says ping." {
		t.Fatalf("the final answer = %q", last)
	}
	if runs.Load() != 1 {
		t.Fatalf("tool runs = %d", runs.Load())
	}
	second := backend.Requests()[1]
	if !strings.Contains(second, `"role":"tool"`) || !strings.Contains(second, `"tool_call_id":"call_1"`) || !strings.Contains(second, `"tools"`) {
		t.Fatalf("the second request does not carry the tool result: %s", second)
	}
	if f.settled[len(f.settled)-1].Ending != EndFinished {
		t.Fatalf("the turn ended %+v", f.settled[len(f.settled)-1])
	}
}

// TestStepLimitKeepsTheTranscriptAndNamesTheLimit runs a model that asks for a tool on every turn.
func TestStepLimitKeepsTheTranscriptAndNamesTheLimit(t *testing.T) {
	var runs atomic.Int32
	var replies []fixtures.ScriptedReply
	for i := 0; i < agent.DefaultMaxSteps; i++ {
		replies = append(replies, fixtures.ToolCallReply("", fixtures.ToolCall{ID: "loop", Name: "echo", Arguments: []string{`{"text":"again"}`}}))
	}
	f := newScriptedFixture(t, toolsOf(t, echoTool(&runs)), fixtures.NewScriptedBackend(replies...))
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, "loop"); err != nil {
		t.Fatal(err)
	}
	f.awaitTurn(chat.ID)
	if runs.Load() != agent.DefaultMaxSteps {
		t.Fatalf("tool runs = %d, want %d", runs.Load(), agent.DefaultMaxSteps)
	}
	if kinds := noticeKinds(f.messages(chat.ID)); len(kinds) != 1 || kinds[0] != NoticeStepLimit {
		t.Fatalf("notices = %v", kinds)
	}
}

// TestRateLimitAfterAToolKeepsItsRow checks that a failure after a tool result keeps the row and names the failure.
func TestRateLimitAfterAToolKeepsItsRow(t *testing.T) {
	var runs atomic.Int32
	backend := fixtures.NewScriptedBackend(
		fixtures.ToolCallReply("", fixtures.ToolCall{ID: "e1", Name: "echo", Arguments: []string{`{"text":"x"}`}}),
		fixtures.ScriptedReply{Status: 429, ErrorType: "rate_limited"},
	)
	f := newScriptedFixture(t, toolsOf(t, echoTool(&runs)), backend)
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, "x"); err != nil {
		t.Fatal(err)
	}
	f.awaitTurn(chat.ID)
	messages := f.messages(chat.ID)
	if !hasToolWith(messages, StatusComplete) {
		t.Fatalf("the tool row lost its result: %+v", toolRows(messages))
	}
	if kinds := noticeKinds(messages); len(kinds) != 1 || kinds[0] != NoticeRateLimited {
		t.Fatalf("notices = %v", kinds)
	}
}

// TestServerErrorAfterToolKeepsRowAndNamesTheFailure checks a 5xx from a follow-up model turn.
func TestServerErrorAfterToolKeepsRowAndNamesTheFailure(t *testing.T) {
	var runs atomic.Int32
	backend := fixtures.NewScriptedBackend(
		fixtures.ToolCallReply("", fixtures.ToolCall{ID: "e1", Name: "echo", Arguments: []string{`{"text":"x"}`}}),
		fixtures.ScriptedReply{Status: 500, ErrorType: "server_error"},
	)
	f := newScriptedFixture(t, toolsOf(t, echoTool(&runs)), backend)
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, "x"); err != nil {
		t.Fatal(err)
	}
	f.awaitTurn(chat.ID)
	messages := f.messages(chat.ID)
	if !hasToolWith(messages, StatusComplete) {
		t.Fatalf("the tool row lost its result: %+v", toolRows(messages))
	}
	if kinds := noticeKinds(messages); len(kinds) != 1 || kinds[0] != NoticeFailed {
		t.Fatalf("notices = %v", kinds)
	}
}

// TestBrokenStreamAfterToolCallIsNotAFinishedReply checks a call whose stream breaks before its end marker: the call
// is not run, and the turn ends as a failed reply with its notice.
func TestBrokenStreamAfterToolCallIsNotAFinishedReply(t *testing.T) {
	var runs atomic.Int32
	broken := fixtures.ToolCallReply("", fixtures.ToolCall{ID: "b1", Name: "echo", Arguments: []string{`{"text":"x"}`}})
	broken.Frames = broken.Frames[:len(broken.Frames)-1]
	backend := fixtures.NewScriptedBackend(broken)
	f := newScriptedFixture(t, toolsOf(t, echoTool(&runs)), backend)
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, "x"); err != nil {
		t.Fatal(err)
	}
	f.awaitTurn(chat.ID)
	if f.settled[len(f.settled)-1].Ending != EndFailed {
		t.Fatalf("the turn ended %+v, want failed", f.settled[len(f.settled)-1])
	}
	if kinds := noticeKinds(f.messages(chat.ID)); len(kinds) != 1 || kinds[0] != NoticeFailed && kinds[0] != NoticeCutOff {
		t.Fatalf("notices = %v", kinds)
	}
}

// TestCreditsRunOutAfterAToolIsNamed checks the 402 path from a follow-up model turn.
func TestCreditsRunOutAfterAToolIsNamed(t *testing.T) {
	var runs atomic.Int32
	backend := fixtures.NewScriptedBackend(
		fixtures.ToolCallReply("", fixtures.ToolCall{ID: "e1", Name: "echo", Arguments: []string{`{"text":"x"}`}}),
		fixtures.ScriptedReply{Status: 402, ErrorType: "insufficient_credits"},
	)
	f := newScriptedFixture(t, toolsOf(t, echoTool(&runs)), backend)
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, "x"); err != nil {
		t.Fatal(err)
	}
	f.awaitTurn(chat.ID)
	if kinds := noticeKinds(f.messages(chat.ID)); len(kinds) != 1 || kinds[0] != NoticeCredits {
		t.Fatalf("notices = %v", kinds)
	}
}

// TestMalformedArgumentsBecomeAFailedRowAndTheTurnGoesOn checks a call whose arguments do not parse.
func TestMalformedArgumentsBecomeAFailedRowAndTheTurnGoesOn(t *testing.T) {
	var runs atomic.Int32
	backend := fixtures.NewScriptedBackend(
		fixtures.ToolCallReply("", fixtures.ToolCall{ID: "m1", Name: "echo", Arguments: []string{`{"text":`}}),
		fixtures.TextReply("I could not run it."),
	)
	f := newScriptedFixture(t, toolsOf(t, echoTool(&runs)), backend)
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, "x"); err != nil {
		t.Fatal(err)
	}
	f.awaitTurn(chat.ID)
	rows := toolRows(f.messages(chat.ID))
	if runs.Load() != 0 || len(rows) != 1 || rows[0].Status != StatusFailed || rows[0].Tool.Outcome != "Invalid arguments" {
		t.Fatalf("rows = %+v, runs = %d", rows, runs.Load())
	}
	if f.settled[len(f.settled)-1].Ending != EndFinished {
		t.Fatalf("the turn ended %+v", f.settled[len(f.settled)-1])
	}
}

// TestBuilderGetsNoTools checks that the extension builder never offers the conversation's tools.
func TestBuilderGetsNoTools(t *testing.T) {
	var runs atomic.Int32
	backend := fixtures.NewScriptedBackend(fixtures.TextReply("Here it is."))
	f := newScriptedFixture(t, toolsOf(t, echoTool(&runs)), backend)
	builder, err := f.svc.NewChat(KindBuilder, TierSmart)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Send(builder.ID, "make one"); err != nil {
		t.Fatal(err)
	}
	f.awaitTurn(builder.ID)
	if strings.Contains(backend.Requests()[0], `"tools"`) {
		t.Fatal("the builder was offered tools")
	}
}

// kindsOf names the authors of a transcript, in order, for sequence assertions.
func kindsOf(messages []Message) string {
	names := make([]string, 0, len(messages))
	for _, message := range messages {
		names = append(names, string(message.Author))
	}
	return strings.Join(names, " ")
}

// chatOf returns the chat with id, failing the test when it is gone.
func (f *fixture) chatOf(id string) Chat {
	f.t.Helper()
	chat, ok := f.svc.Chat(id)
	if !ok {
		f.t.Fatalf("chat %s is gone", id)
	}
	return chat
}
