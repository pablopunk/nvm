package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestSummonAfterHideShowsTheRootOnly checks that pages pushed above the root do not survive a hide: the next summon
// starts on the root, as Electron's onShown clears the stack of a palette whose AI chat was not open.
func TestSummonAfterHideShowsTheRootOnly(t *testing.T) {
	var ran []string
	root := rowsView("Nevermind", &ran, "Settings")
	h := newHarness(t, root, true)
	settings := rowsView("Settings", &ran, "Account")
	account := rowsView("Nevermind Account", &ran, "Sign in")
	h.p.Push(settings)
	h.p.Push(account)

	h.p.Hide()
	h.p.Show()
	h.frame()

	if h.p.Top() != root || len(h.p.stack) != 1 {
		t.Fatalf("summon after hide should show the root alone: top %q, stack depth %d", h.p.Top().Title, len(h.p.stack))
	}
}

// TestHideKeepsTheLiveChatAndDropsWhatWasPushedAboveIt checks the one page that survives a hide. Electron's hidePanes
// keeps the frames up to the last AI chat and drops what was opened above it, and the next summon shows that chat.
func TestHideKeepsTheLiveChatAndDropsWhatWasPushedAboveIt(t *testing.T) {
	var ran []string
	root := rowsView("Nevermind", &ran, "AI Chat")
	h := newHarness(t, root, true)
	chat := rowsView("AI Chat", &ran, "Reply")
	chat.SurvivesHide = true
	settings := rowsView("Settings", &ran, "Account")
	h.p.Push(chat)
	h.p.Push(settings)

	h.p.Hide()
	h.p.Show()
	h.frame()

	if h.p.Top() != chat || len(h.p.stack) != 2 || h.p.stack[0] != root {
		t.Fatalf("summon after hiding over a chat should show the chat over the root: top %q, stack depth %d", h.p.Top().Title, len(h.p.stack))
	}
}

// TestEscapeAsksThePageFirst checks that a page's OnEscape sees Escape before the palette goes back: a page that used
// it stays, and a page that did not lets Escape go back as it does everywhere else.
func TestEscapeAsksThePageFirst(t *testing.T) {
	var ran []string
	root := rowsView("Nevermind", &ran, "Open")
	h := newHarness(t, root, true)
	used := true
	page := rowsView("Page", &ran, "Row")
	page.OnEscape = func() bool { return used }
	h.p.Push(page)

	h.key(0, ui.KeyEscape)
	if h.p.Top() != page {
		t.Fatal("Escape the page used should keep the page open")
	}

	used = false
	h.key(0, ui.KeyEscape)
	if h.p.Top() != root {
		t.Fatal("Escape the page did not use should go back to the root")
	}
}
