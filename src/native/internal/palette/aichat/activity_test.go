package aichat

import (
	"reflect"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/search"
)

func activityTool(name string, status chat.Status) chat.Message {
	return chat.Message{Author: chat.AuthorTool, Status: status, Tool: &chat.ToolRun{Name: name, Label: name, Outcome: "Private tool result"}}
}

func TestToolProtocolHistoryIsNeverConversationContent(t *testing.T) {
	for _, status := range []chat.Status{chat.StatusComplete, chat.StatusFailed, chat.StatusRefused, chat.StatusCancelled} {
		messages := []chat.Message{user("Build a toggle"), activityTool("read_extension_api", status), {Author: chat.AuthorAssistant, Text: "The toggle is ready.", Status: chat.StatusComplete}}
		got := presentChat(messages)
		if !reflect.DeepEqual(got.visible, []int{0, 2}) || got.activity != -1 || got.busy {
			t.Fatalf("%s presentation = %+v", status, got)
		}
		if messages[1].Tool.Outcome != "Private tool result" {
			t.Fatal("presentation mutated model history")
		}
	}
}

func TestCurrentToolReplacesEarlierActivityAndApprovalRemainsActionable(t *testing.T) {
	x := NewExtensionChat(nil, func() time.Time { return goldenNow })
	messages := []chat.Message{user("Build a toggle"), activityTool("read_extension_api", chat.StatusComplete), activityTool("write_extension", chat.StatusRunning)}
	x.SetSpec(ExtensionChatSpec{Messages: messages})
	if x.presentation.activity != 2 || !x.streaming() || x.awaitingAnswer() {
		t.Fatalf("running state = %+v", x.presentation)
	}
	messages[2].Status = chat.StatusAwaiting
	x.SetSpec(ExtensionChatSpec{Messages: messages})
	if x.streaming() || !x.awaitingAnswer() || !x.canSend("") {
		t.Fatalf("approval state = %+v", x.presentation)
	}
	messages = append(messages, chat.Message{Author: chat.AuthorNotice, Notice: &chat.Notice{Title: "Stopped", Message: "The request stopped"}})
	x.SetSpec(ExtensionChatSpec{Messages: messages})
	if x.presentation.activity != -1 || x.streaming() || x.awaitingAnswer() {
		t.Fatalf("terminal state = %+v", x.presentation)
	}
}

func TestLiveActivityTransitionsSettleAndRespectReducedMotion(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		x := NewExtensionChat(nil, func() time.Time { return goldenNow })
		tester := ui.NewTester(func(c *ui.Context) { ui.Column(c).Fill().Children(func() { x.activity(c) }) }, 360, 100)
		tester.SetPreferences(ui.Preferences{ReduceMotion: reduced})
		x.SetSpec(ExtensionChatSpec{Messages: []chat.Message{activityTool("read_extension_api", chat.StatusAwaiting)}})
		tester.Frame()
		if !tester.HasText("Reading the extension API") {
			t.Fatal("live tool was not human readable")
		}
		x.SetSpec(ExtensionChatSpec{Messages: []chat.Message{activityTool("write_extension", chat.StatusAwaiting)}})
		tester.Frame()
		if !tester.HasText("Writing the extension") {
			t.Fatal("next activity did not replace the previous one")
		}
		if reduced && tester.FramePending() {
			t.Fatal("reduced-motion tool status kept animating")
		}
		tester.Advance(300 * time.Millisecond)
		if tester.FramePending() {
			t.Fatal("status replacement did not settle")
		}
		x.SetSpec(ExtensionChatSpec{Messages: []chat.Message{activityTool("write_extension", chat.StatusComplete)}})
		tester.Frame()
		tester.Advance(300 * time.Millisecond)
		if tester.HasText("Writing the extension") || tester.FramePending() {
			t.Fatal("completed activity or its frame loop remained")
		}
	}
}

func TestLiveActivityChatGoldens(t *testing.T) {
	for _, state := range []string{"running", "approval", "settled"} {
		t.Run(state, func(t *testing.T) {
			r := newRig(t, nil)
			x := NewExtensionChat(func(c *ui.Context, text string, _ bool) { ui.Text(c, text).FontSize(13) }, func() time.Time { return goldenNow })
			messages := []chat.Message{user("Build a Tailscale toggle"), {Author: chat.AuthorAssistant, Text: "I’ll use the current settings for the toggle.", Status: chat.StatusComplete}, activityTool("read_extension_api", chat.StatusComplete), activityTool("write_extension", chat.StatusRunning)}
			if state == "approval" {
				messages[3].Status = chat.StatusAwaiting
				messages[3].Tool.Outcome = "Allow creating the Tailscale toggle?"
			}
			if state == "settled" {
				messages[3].Status = chat.StatusComplete
				messages = append(messages, chat.Message{Author: chat.AuthorAssistant, Text: "The Tailscale toggle is ready.", Status: chat.StatusComplete})
			}
			x.SetSpec(ExtensionChatSpec{Title: "Tailscale toggle", Messages: messages})
			r.open(x.View())
			golden.AssertMatrix(t, "tool-activity-"+state, r.palette.View, pageGoldens)
		})
	}
}

func TestApprovalWinsOverConcurrentActivityAndAcceptsEnter(t *testing.T) {
	sent := false
	x := NewExtensionChat(nil, func() time.Time { return goldenNow })
	x.SetSpec(ExtensionChatSpec{Messages: []chat.Message{activityTool("read_clipboard", chat.StatusAwaiting), activityTool("search_files", chat.StatusRunning)}, Send: func(search.Host, string, []chat.Image) { sent = true }})
	if x.presentation.activity != 0 || !x.send("") || !sent {
		t.Fatal("concurrent activity hid or blocked the approval")
	}
}
