package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/extai"
	"github.com/pablopunk/nvm/native/internal/ai/extai/extaitest"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// aiHandler returns the services of an extension that declares capabilities, over the AI rig.
func aiHandler(t *testing.T, rig *extaitest.Rig, capabilities ...api.Capability) *Handler {
	t.Helper()
	handler := New(Deps{Clock: rig.Set.Clock, DataRoot: t.TempDir(), AI: rig.Service},
		api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "acme.ai", Title: "AI"}, Capabilities: capabilities})
	t.Cleanup(func() { _ = handler.Close() })
	return handler
}

// aiCall answers one ai operation of handler, as the child's call would be answered.
func aiCall(t *testing.T, handler *Handler, op string, args any) protocol.ServiceReply {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("arguments: %v", err)
	}
	return handler.Serve(context.Background(), protocol.ServiceCall{Service: protocol.ServiceAI, Op: op, Args: raw})
}

// askText asks once and returns the text, failing the test on a failure.
func askText(t *testing.T, handler *Handler, prompt string) string {
	t.Helper()
	reply := aiCall(t, handler, protocol.OpAsk, protocol.AIAskArgs{Prompt: prompt})
	if reply.Failure != nil {
		t.Fatalf("ask failed: %+v", reply.Failure)
	}
	var text string
	if err := json.Unmarshal(reply.Data, &text); err != nil {
		t.Fatalf("ask answer: %v", err)
	}
	return text
}

func TestAIIsRefusedWithoutTheCapability(t *testing.T) {
	rig := extaitest.New(t)
	reply := aiCall(t, aiHandler(t, rig), protocol.OpAsk, protocol.AIAskArgs{Prompt: "hi"})
	if reply.Failure == nil || reply.Failure.Code != api.CodePermission || !strings.Contains(reply.Failure.Message, "ai") {
		t.Fatalf("failure %+v; want a permission failure naming the ai capability", reply.Failure)
	}
	if len(rig.Backend.Requests()) != 0 {
		t.Fatal("a refused call reached the backend")
	}
}

func TestAskReturnsTheReply(t *testing.T) {
	rig := extaitest.New(t)
	rig.Backend.SetReply("pong")
	if got := askText(t, aiHandler(t, rig, api.CapabilityAI), "ping"); got != "pong" {
		t.Fatalf("ask returned %q; want pong", got)
	}
}

func TestAskRateLimitIsThirtyAMinute(t *testing.T) {
	rig := extaitest.New(t)
	handler := aiHandler(t, rig, api.CapabilityAI)
	for i := 0; i < 30; i++ {
		askText(t, handler, "again")
	}
	reply := aiCall(t, handler, protocol.OpAsk, protocol.AIAskArgs{Prompt: "one too many"})
	if reply.Failure == nil || reply.Failure.Code != api.CodeQuota || !strings.Contains(reply.Failure.Message, "AI rate limit exceeded") {
		t.Fatalf("31st call failure %+v; want the quota failure in 1.x words", reply.Failure)
	}
	rig.Set.Clock.Advance(time.Minute)
	askText(t, handler, "after a minute")
}

func TestSignedOutAsksFailAsUnavailable(t *testing.T) {
	rig := extaitest.New(t)
	rig.SignOut()
	reply := aiCall(t, aiHandler(t, rig, api.CapabilityAI), protocol.OpAsk, protocol.AIAskArgs{Prompt: "hi"})
	if reply.Failure == nil || reply.Failure.Code != api.CodeUnavailable || !strings.Contains(reply.Failure.Message, "Sign in") {
		t.Fatalf("failure %+v; want an unavailable failure asking for sign-in", reply.Failure)
	}
}

// pullAll pulls a stream until it ends and returns its events and final batch.
func pullAll(t *testing.T, handler *Handler, id string) ([]api.AIStreamEvent, protocol.AIStreamBatch) {
	t.Helper()
	var events []api.AIStreamEvent
	for {
		reply := aiCall(t, handler, protocol.OpStreamNext, protocol.AIStreamRef{ID: id})
		if reply.Failure != nil {
			t.Fatalf("next failed: %+v", reply.Failure)
		}
		var batch protocol.AIStreamBatch
		if err := json.Unmarshal(reply.Data, &batch); err != nil {
			t.Fatalf("batch: %v", err)
		}
		events = append(events, batch.Events...)
		if batch.Ended {
			return events, batch
		}
	}
}

func openStream(t *testing.T, handler *Handler, prompt string) string {
	t.Helper()
	reply := aiCall(t, handler, protocol.OpStreamOpen, protocol.AIAskArgs{Prompt: prompt})
	if reply.Failure != nil {
		t.Fatalf("open failed: %+v", reply.Failure)
	}
	var opened protocol.AIStreamOpened
	if err := json.Unmarshal(reply.Data, &opened); err != nil {
		t.Fatalf("opened: %v", err)
	}
	return opened.ID
}

func TestStreamPullDeliversEventsInOrder(t *testing.T) {
	rig := extaitest.New(t)
	rig.Backend.SetReply("the streamed reply")
	handler := aiHandler(t, rig, api.CapabilityAI)
	events, last := pullAll(t, handler, openStream(t, handler, "stream"))
	if len(events) < 2 || events[0].Type != api.AIEventStart || events[len(events)-1].Type != api.AIEventDone {
		t.Fatalf("events %+v; want start first and done last", events)
	}
	var text strings.Builder
	for _, event := range events {
		if event.Type == api.AIEventDelta {
			text.WriteString(event.Text)
		}
	}
	if text.String() != "the streamed reply" || last.Text != "the streamed reply" || last.Failure != nil {
		t.Fatalf("text %q, final %q; want the whole reply without failure", text.String(), last.Text)
	}
}

func TestSessionKeepsTheEarlierTurns(t *testing.T) {
	rig := extaitest.New(t)
	handler := aiHandler(t, rig, api.CapabilityAI)
	ask := func(prompt string) {
		reply := aiCall(t, handler, protocol.OpAsk, protocol.AIAskArgs{Prompt: prompt, Session: "chat"})
		if reply.Failure != nil {
			t.Fatalf("session ask failed: %+v", reply.Failure)
		}
	}
	ask("first question")
	ask("second question")
	requests := rig.Backend.Requests()
	last := requests[len(requests)-1].Body
	if !strings.Contains(last, "first question") || !strings.Contains(last, "second question") {
		t.Fatalf("second turn body %s; want the first turn sent with the second", last)
	}
}

func TestTimedOutAskFailsAsTimeout(t *testing.T) {
	rig := extaitest.New(t)
	rig.Backend.SetMode(fixtures.ModeTimeout)
	handler := aiHandler(t, rig, api.CapabilityAI)
	result := make(chan protocol.ServiceReply, 1)
	go func() { result <- aiCall(t, handler, protocol.OpAsk, protocol.AIAskArgs{Prompt: "hang"}) }()
	<-rig.Backend.Hanging()
	rig.Set.Clock.AwaitTimers(1)
	rig.Set.Clock.Advance(extai.CallTimeout)
	if reply := <-result; reply.Failure == nil || reply.Failure.Code != api.CodeTimeout {
		t.Fatalf("failure %+v; want a timeout failure", reply.Failure)
	}
}

func TestFileAttachmentNeedsTheFilesCapability(t *testing.T) {
	rig := extaitest.New(t)
	args := protocol.AIAskArgs{Prompt: "read", Options: api.AIOptions{Attachments: []api.Attachment{{Kind: "file", Path: "/home/ada/notes.md"}}}}
	reply := aiCall(t, aiHandler(t, rig, api.CapabilityAI), protocol.OpAsk, args)
	if reply.Failure == nil || reply.Failure.Code != api.CodePermission || !strings.Contains(reply.Failure.Message, "desktop.files") {
		t.Fatalf("failure %+v; want a permission failure naming desktop.files", reply.Failure)
	}
}

func TestClipboardAttachmentNeedsTheClipboardCapability(t *testing.T) {
	rig := extaitest.New(t)
	reply := aiCall(t, aiHandler(t, rig, api.CapabilityAI), protocol.OpAttachClipboard, protocol.AIClipboardArgs{As: "text"})
	if reply.Failure == nil || reply.Failure.Code != api.CodePermission {
		t.Fatalf("failure %+v; want a permission failure for the clipboard", reply.Failure)
	}
}
