package aichat

import (
	"io"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// keyToken is the bearer the test client carries.
type keyToken string

func (t keyToken) Get() (string, error) { return string(t), nil }

// streamingClient is a backend client whose chat replies the test writes by hand, or whose answers the transport makes.
func streamingClient(t *testing.T, transport platform.HTTP) *backend.Client {
	t.Helper()
	set := fake.NewSet()
	logger, err := logx.New(set.FS, set.Clock, "/logs", logx.Options{FlushInterval: time.Second, Level: logx.Debug})
	if err != nil {
		t.Fatalf("logx: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	env, err := backend.NewEnvironment(backend.EnvironmentProduction, "", false)
	if err != nil {
		t.Fatalf("environment: %v", err)
	}
	client, err := backend.New(transport, logger, env, keyToken("test-token"),
		backend.WithClock(set.Clock), backend.WithRetryPolicy(1, 0, 0))
	if err != nil {
		t.Fatalf("backend: %v", err)
	}
	return client
}

// awaitReplyText settles queued work until the newest assistant message of the chat reads want.
func (r *rig) awaitReplyText(id, want string) {
	r.t.Helper()
	deadline := time.After(watchdog)
	for {
		r.settle()
		if record, _ := r.service.Chat(id); lastReply(record) == want {
			return
		}
		select {
		case <-r.notify:
		case <-deadline:
			r.t.Fatalf("reply never reached %q", want)
		}
	}
}

// lastReply returns the text of the newest assistant message of record.
func lastReply(record chat.Chat) string {
	text, _ := chat.LastReply(record)
	return text
}

// TestEscapeStopsTheReplyAndKeepsThePage types a question, lets the first words of the answer arrive, and presses
// Escape. The reply stops with the words it had, and the palette stays on the chat instead of closing it.
func TestEscapeStopsTheReplyAndKeepsThePage(t *testing.T) {
	transport := fixtures.NewStreamingTransport()
	r := newRig(t, streamingClient(t, transport))
	record := r.chatWith("", chat.TierFast)
	view := ChatView(r.deps(), record.ID)
	r.palette.Show()
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Type("tell me a story")
	tt.Key(0, ui.KeyEnter)
	reply := awaitReply(t, transport)
	writeFrame(t, reply, fixtures.StreamFrames("Once upon", 24)[0])
	r.awaitReplyText(record.ID, "Once upon")

	tt.Key(0, ui.KeyEscape)
	r.awaitIdle(record.ID)
	if r.palette.Top() != view {
		t.Fatal("Escape during a reply should stop it and keep the chat open")
	}
	messages := mustChat(t, r, record.ID).Messages
	if len(messages) != 2 || messages[1].Text != "Once upon" || messages[1].Status != chat.StatusStopped {
		t.Fatalf("transcript = %+v, want the partial reply marked stopped", messages)
	}

	tt.Key(0, ui.KeyEscape)
	if r.palette.Top() != r.root {
		t.Fatal("with the reply stopped, Escape should go back to the root")
	}
}

// TestEscapeOnAnIdleChatGoesBack checks that Escape pops a chat with no reply in flight, as Electron's
// popExtensionView does. The chat page used to take the key as a stop request even with nothing to stop, and
// the palette never saw it.
func TestEscapeOnAnIdleChatGoesBack(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Idle", chat.TierFast, user("q"), assistant("a"))
	r.palette.Show()
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Key(0, ui.KeyEscape)
	if r.palette.Top() != r.root {
		t.Fatal("Escape on an idle chat should go back to the root")
	}
}

// TestCopyShortcutCopiesTheLastReply checks Command or Control with Shift and C copies the newest reply, and that
// it says so on the palette.
func TestCopyShortcutCopiesTheLastReply(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Copy me", chat.TierFast, user("q"), assistant("The answer to copy."))
	r.palette.Show()
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Key(palette.Mod|ui.Alt, ui.KeyC)
	if len(r.copied) != 1 || r.copied[0] != "The answer to copy." {
		t.Fatalf("copied = %q, want the last reply", r.copied)
	}
}

// TestModelShortcutSwitchesTheTier checks the tier flips between fast and smart and the next reply uses it.
func TestModelShortcutSwitchesTheTier(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Model", chat.TierFast)
	r.palette.Show()
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Key(palette.Mod|ui.Shift, ui.KeyM)
	if got := mustChat(t, r, record.ID).Tier; got != chat.TierSmart {
		t.Fatalf("tier = %s, want smart after one switch", got)
	}
	tt.Key(palette.Mod|ui.Shift, ui.KeyM)
	if got := mustChat(t, r, record.ID).Tier; got != chat.TierFast {
		t.Fatalf("tier = %s, want fast after a second switch", got)
	}
}

// TestHistoryFiltersByTitle checks the chat list narrows to the conversations whose titles match, and keeps the
// new-chat row.
func TestHistoryFiltersByTitle(t *testing.T) {
	r := newRig(t, nil)
	r.chatWith("Plan a weekend", chat.TierFast, user("plan"))
	r.chatWith("Debug the sync", chat.TierFast, user("debug"))
	r.open(HistoryView(r.deps()))
	r.palette.SetQuery("plan")
	titles := resultTitles(r.palette.Results())
	if !contains(titles, "Plan a weekend") || contains(titles, "Debug the sync") || !contains(titles, "New Chat") {
		t.Fatalf("results for plan = %v, want the matching chat and the new-chat row", titles)
	}
}

// awaitReply returns the writer of the next chat reply, failing the test when none arrives.
func awaitReply(t *testing.T, transport *fixtures.StreamingTransport) io.WriteCloser {
	t.Helper()
	select {
	case writer := <-transport.Replies():
		return writer
	case <-time.After(watchdog):
		t.Fatal("no reply was requested")
	}
	return nil
}

// writeFrame sends one frame of a reply.
func writeFrame(t *testing.T, writer io.Writer, frame string) {
	t.Helper()
	if _, err := io.WriteString(writer, frame); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

// mustChat returns the chat with id, failing the test when it is gone.
func mustChat(t *testing.T, r *rig, id string) chat.Chat {
	t.Helper()
	record, ok := r.service.Chat(id)
	if !ok {
		t.Fatalf("chat %s is gone", id)
	}
	return record
}

// resultTitles lists the titles of the rows.
func resultTitles(items []search.Item) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Title)
	}
	return out
}

// contains reports whether want is in titles.
func contains(titles []string, want string) bool {
	for _, title := range titles {
		if title == want {
			return true
		}
	}
	return false
}

// TestStopButtonShowsWhileAReplyStreams checks the Stop control is on the page while a reply streams, and that a click
// stops the reply and takes the control away.
func TestStopButtonShowsWhileAReplyStreams(t *testing.T) {
	transport := fixtures.NewStreamingTransport()
	r := newRig(t, streamingClient(t, transport))
	record := r.chatWith("Counting", chat.TierFast)
	r.palette.Show()
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	if contains(tt.Texts(), "Stop") {
		t.Fatal("Stop is on the page before a reply starts")
	}

	tt.Type("count to ten")
	tt.Key(0, ui.KeyEnter)
	reply := awaitReply(t, transport)
	writeFrame(t, reply, fixtures.StreamFrames("One, two", 24)[0])
	r.awaitReplyText(record.ID, "One, two")
	tt.Frame()
	if !contains(tt.Texts(), "Stop") {
		t.Fatal("Stop is not on the page while the reply streams")
	}

	if err := tt.Click("Stop"); err != nil {
		t.Fatalf("click Stop: %v", err)
	}
	r.awaitIdle(record.ID)
	tt.Frame()
	if contains(tt.Texts(), "Stop") {
		t.Fatal("Stop stays on the page after the reply stopped")
	}
}

// TestTierPickerSwitchesTheTierOnClick checks a click on the tier picker moves the chat to the other tier, and that a
// click while a reply streams keeps the tier and says why.
func TestTierPickerSwitchesTheTierOnClick(t *testing.T) {
	transport := fixtures.NewStreamingTransport()
	r := newRig(t, streamingClient(t, transport))
	record := r.chatWith("Tiers", chat.TierFast)
	r.palette.Show()
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	if err := tt.Click("Fast ⌄"); err != nil {
		t.Fatalf("click the tier picker: %v", err)
	}
	if got := mustChat(t, r, record.ID).Tier; got != chat.TierSmart {
		t.Fatalf("tier = %s after a click, want smart", got)
	}

	if err := r.service.Send(record.ID, "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	reply := awaitReply(t, transport)
	writeFrame(t, reply, fixtures.StreamFrames("Hi", 24)[0])
	r.awaitReplyText(record.ID, "Hi")
	tt.Frame()
	if err := tt.Click("Smart ⌄"); err != nil {
		t.Fatalf("click the tier picker while busy: %v", err)
	}
	if got := mustChat(t, r, record.ID).Tier; got != chat.TierSmart {
		t.Fatalf("tier = %s while a reply streams, want it unchanged", got)
	}
}
