package palette

import "testing"

// TestNoticeShowsItsTextAndRunsItsButton checks the banner: its title and button are on screen, and pressing the
// button runs the action once, leaving the list as it was.
func TestNoticeShowsItsTextAndRunsItsButton(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true)
	pressed := 0
	h.p.SetNotice(&Notice{
		Icon:        "keyboard",
		Title:       "Alt+Shift+Space is unavailable",
		Detail:      "Use Control+Alt+K to reopen Nevermind.",
		ActionLabel: "Change shortcut",
		Alert:       true,
		Action:      func() { pressed++ },
	})
	h.frame()
	if !h.tt.HasText("Alt+Shift+Space is unavailable") || !h.tt.HasText("Change shortcut") {
		t.Fatalf("the banner is not on screen; texts = %v", h.tt.Texts())
	}
	if err := h.tt.Click("Change shortcut"); err != nil {
		t.Fatalf("clicking the banner button: %v", err)
	}
	h.frame()
	if pressed != 1 {
		t.Fatalf("the action ran %d times, want once", pressed)
	}
	if got := h.titles(); len(got) != 1 || got[0] != "Alpha" {
		t.Fatalf("rows = %v; the banner must not change the results", got)
	}
}

// TestNoticeClearsWhenNil checks that setting no notice removes the banner.
func TestNoticeClearsWhenNil(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true)
	h.p.SetNotice(&Notice{Title: "No shortcut set", ActionLabel: "Set shortcut"})
	h.frame()
	h.p.SetNotice(nil)
	h.frame()
	if h.tt.HasText("No shortcut set") {
		t.Fatal("the banner is still on screen after SetNotice(nil)")
	}
}

// TestAlertNoticeIsAnnouncedOnce checks J27 for the hotkey banner: an alert is announced when it appears, and
// redrawing it does not announce it again.
func TestAlertNoticeIsAnnouncedOnce(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true)
	h.p.SetNotice(&Notice{Title: "Alt+Shift+Space is unavailable", Detail: "Use Control+Alt+K to reopen Nevermind.", ActionLabel: "Change shortcut", Alert: true})
	h.frame()
	h.frame()
	count := 0
	for _, text := range h.tt.Announcements() {
		if text == "Alt+Shift+Space is unavailable. Use Control+Alt+K to reopen Nevermind." {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("the alert was announced %d times, want once; announcements = %v", count, h.tt.Announcements())
	}
}
