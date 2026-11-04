package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
)

// rememberText puts one text entry in the history, so a destructive call has something to remove.
func rememberText(t *testing.T, history *clipboard.History, text string) {
	t.Helper()
	now := time.Now()
	if err := history.Remember(clipboard.Entry{ID: clipboard.TextID(text), Kind: clipboard.KindText, Text: text, CreatedAt: now, UsedAt: now}); err != nil {
		t.Fatalf("remember %q: %v", text, err)
	}
}

// answering returns a ConfirmFunc that records each question and gives the answer, or the error, it was built with.
func answering(answer bool, err error, asked *[]Question) ConfirmFunc {
	return func(_ context.Context, question Question) (bool, error) {
		*asked = append(*asked, question)
		return answer, err
	}
}

func TestClearHistoryRunsOnlyAfterTheUserConfirms(t *testing.T) {
	host, deps := newDesktopHost(t)
	rememberText(t, host.history, "draft")
	var asked []Question
	deps.Confirm = answering(true, nil, &asked)
	h := desktopHandler(t, deps, api.CapabilityClipboardHistory)
	reply := serveRun(t, h, protocol.ServiceClipboardHistory, protocol.OpClear, protocol.ClearArgs{})
	var removed int
	if err := jsonInto(mustOK(t, reply), &removed); err != nil || removed != 1 {
		t.Fatalf("removed %d (%v), want 1 after confirm", removed, err)
	}
	if len(asked) != 1 || asked[0].Action != "Clear clipboard history" || asked[0].Extension != "acme.desktop" {
		t.Fatalf("questions %+v", asked)
	}
	if !reply.WaitedForUser {
		t.Fatal("a confirmed clear did not report that it waited for the user")
	}
	if host.history.Len() != 0 {
		t.Fatalf("history still holds %d entries", host.history.Len())
	}
}

func TestCancelledClearLeavesHistoryUntouched(t *testing.T) {
	host, deps := newDesktopHost(t)
	rememberText(t, host.history, "draft")
	var asked []Question
	deps.Confirm = answering(false, nil, &asked)
	h := desktopHandler(t, deps, api.CapabilityClipboardHistory)
	reply := serveRun(t, h, protocol.ServiceClipboardHistory, protocol.OpClear, protocol.ClearArgs{})
	requireCode(t, reply, api.CodePermission, "cancelled")
	if host.history.Len() != 1 {
		t.Fatalf("a cancelled clear removed entries: %d left", host.history.Len())
	}
}

func TestRemoveNeedsTheUsersAnswerToo(t *testing.T) {
	host, deps := newDesktopHost(t)
	rememberText(t, host.history, "draft")
	id := clipboard.TextID("draft")
	var asked []Question
	deps.Confirm = answering(false, nil, &asked)
	h := desktopHandler(t, deps, api.CapabilityClipboardHistory)
	requireCode(t, serveRun(t, h, protocol.ServiceClipboardHistory, protocol.OpRemove, protocol.IDsArgs{IDs: []string{id}}), api.CodePermission, "cancelled")
	if host.history.Len() != 1 || len(asked) != 1 || asked[0].Action != "Remove 1 clipboard items" {
		t.Fatalf("history %d, questions %+v", host.history.Len(), asked)
	}
}

func TestDestructiveCallRefusesWithoutAConfirmationPort(t *testing.T) {
	host, deps := newDesktopHost(t)
	rememberText(t, host.history, "draft")
	deps.Confirm = nil
	h := desktopHandler(t, deps, api.CapabilityClipboardHistory)
	requireCode(t, serveRun(t, h, protocol.ServiceClipboardHistory, protocol.OpClear, protocol.ClearArgs{}), api.CodeUnavailable, "confirmation")
	if host.history.Len() != 1 {
		t.Fatal("a call without a confirmation port removed entries")
	}
}

func TestUnansweredQuestionCancelsTheCall(t *testing.T) {
	host, deps := newDesktopHost(t)
	rememberText(t, host.history, "draft")
	var asked []Question
	deps.Confirm = answering(false, errors.New("the palette closed"), &asked)
	h := desktopHandler(t, deps, api.CapabilityClipboardHistory)
	requireCode(t, serveRun(t, h, protocol.ServiceClipboardHistory, protocol.OpClear, protocol.ClearArgs{}), api.CodeCancelled, "not confirmed in time")
	if host.history.Len() != 1 {
		t.Fatal("an unanswered clear removed entries")
	}
}

func TestSearchCannotAskOrClear(t *testing.T) {
	host, deps := newDesktopHost(t)
	rememberText(t, host.history, "draft")
	var asked []Question
	deps.Confirm = answering(true, nil, &asked)
	h := desktopHandler(t, deps, api.CapabilityClipboardHistory)
	requireCode(t, serve(t, h, protocol.ServiceClipboardHistory, protocol.OpClear, protocol.ClearArgs{}), api.CodePermission, "palette action")
	if len(asked) != 0 || host.history.Len() != 1 {
		t.Fatalf("a search asked %+v or cleared the history", asked)
	}
}

// TestConfirmWaitEndsWhenTheExtensionStops runs a destructive call whose confirmation never gets an answer, then stops
// the extension's lifetime. The question must end at once and refuse the call; it must not wait out ConfirmWait.
func TestConfirmWaitEndsWhenTheExtensionStops(t *testing.T) {
	host, deps := newDesktopHost(t)
	rememberText(t, host.history, "draft")
	lifetime, stop := context.WithCancel(context.Background())
	defer stop()
	asked := make(chan context.Context, 1)
	deps.Confirm = func(ctx context.Context, _ Question) (bool, error) {
		asked <- ctx
		<-ctx.Done()
		return false, ctx.Err()
	}
	h := desktopHandler(t, deps, api.CapabilityClipboardHistory)
	done := make(chan protocol.ServiceReply, 1)
	go func() {
		ctx := protocol.WithLifetime(protocol.WithRequestMethod(context.Background(), protocol.MethodRun), lifetime)
		done <- h.Serve(ctx, protocol.ServiceCall{CallID: 1, Service: protocol.ServiceClipboardHistory, Op: protocol.OpClear})
	}()
	waitCtx := <-asked
	stop()
	select {
	case reply := <-done:
		requireCode(t, reply, api.CodeCancelled, "not confirmed in time")
	case <-time.After(5 * time.Second):
		t.Fatal("the call kept waiting after the extension stopped")
	}
	if waitCtx.Err() == nil {
		t.Fatal("the question's context was not ended")
	}
	if host.history.Len() != 1 {
		t.Fatal("a stopped confirmation removed entries")
	}
}

// TestConfirmWaitIsBoundedByTheConfirmationNotTheRequest checks the question has its own deadline, close to ConfirmWait,
// and not the few seconds a request budget allows.
func TestConfirmWaitIsBoundedByTheConfirmationNotTheRequest(t *testing.T) {
	host, deps := newDesktopHost(t)
	rememberText(t, host.history, "draft")
	var deadline time.Time
	deps.Confirm = func(ctx context.Context, _ Question) (bool, error) {
		deadline, _ = ctx.Deadline()
		return true, nil
	}
	h := desktopHandler(t, deps, api.CapabilityClipboardHistory)
	_ = serveRun(t, h, protocol.ServiceClipboardHistory, protocol.OpClear, protocol.ClearArgs{})
	if remaining := time.Until(deadline); remaining < ConfirmWait-5*time.Second || remaining > ConfirmWait {
		t.Fatalf("the question's deadline is %s away, want about %s", remaining, ConfirmWait)
	}
}
