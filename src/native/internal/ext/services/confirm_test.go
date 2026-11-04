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
