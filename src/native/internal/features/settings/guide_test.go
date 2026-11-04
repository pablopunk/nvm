package settings

import "testing"

// TestGuideEntryIsWelcomeUntilDismissedThenHelp keeps Welcome at the top of the root until the user is done with it.
func TestGuideEntryIsWelcomeUntilDismissedThenHelp(t *testing.T) {
	title, _, score := GuideEntry(false)
	if title != "Welcome to Nevermind" || score != welcomeRootScore {
		t.Fatalf("pending guide = %q at %d, want Welcome at %d", title, score, welcomeRootScore)
	}
	title, _, score = GuideEntry(true)
	if title != "Help" || score != helpRootScore {
		t.Fatalf("done guide = %q at %d, want Help at %d", title, score, helpRootScore)
	}
}

// TestGuideShortcutItemDependsOnTheShortcut offers to set a shortcut, or confirms the one set.
func TestGuideShortcutItemDependsOnTheShortcut(t *testing.T) {
	unset := GuideRows("")
	var shortcut GuideRow
	for _, row := range unset {
		if row.ID == "getting-started-shortcut" {
			shortcut = row
		}
	}
	if shortcut.Title != "Open Nevermind from anywhere" || shortcut.Action != GuideSetShortcut {
		t.Fatalf("unset shortcut row = %+v", shortcut)
	}
	set := GuideRows("Alt+Shift+Space")
	for _, row := range set {
		if row.ID == "getting-started-shortcut" {
			shortcut = row
		}
	}
	if shortcut.Title != "Keyboard shortcut set" || shortcut.Subtitle != "Alt+Shift+Space opens Nevermind from anywhere. You can change it any time." {
		t.Fatalf("set shortcut row = %+v", shortcut)
	}
}

// TestGuideEndsWithDoneForNow lets the user dismiss the guide from its last item.
func TestGuideEndsWithDoneForNow(t *testing.T) {
	rows := GuideRows("")
	last := rows[len(rows)-1]
	if last.ID != "getting-started-done" || last.Action != GuideFinish {
		t.Fatalf("last guide row = %+v, want Done for now", last)
	}
}
