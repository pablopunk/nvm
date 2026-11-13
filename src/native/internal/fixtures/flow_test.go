package fixtures

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// clipboardHistory is the test double for the clipboard history feature: it waits for a change notification,
// reads what is on the clipboard, and records text and images, but never concealed content. The real feature
// lives in a later work package; this checks the port contract it will depend on.
type clipboardHistory struct {
	clipboard platform.Clipboard
	text      []string
	images    int
}

// observeChange waits for one change notification and records what the clipboard now holds. It reports
// whether anything was recorded.
func (h *clipboardHistory) observeChange(t *testing.T) bool {
	t.Helper()
	<-h.clipboard.Changed()
	formats, err := h.clipboard.Formats()
	if err != nil {
		t.Fatalf("Formats: %v", err)
	}
	for _, format := range formats {
		if format == platform.FormatConcealed {
			return false
		}
	}
	for _, format := range formats {
		switch format {
		case platform.FormatText:
			text, err := h.clipboard.ReadText()
			if err != nil {
				t.Fatalf("ReadText: %v", err)
			}
			h.text = append(h.text, text)
			return true
		case platform.FormatPNG:
			h.images++
			return true
		}
	}
	return false
}

// TestCopyingTextReachesHistoryThroughTheChangeEvent drives the clipboard feature at the port level with only
// fakes: the user copies text in another app, the port reports the change, and history shows the text. It also
// covers an image and a password manager's concealed copy, which must never be recorded.
func TestCopyingTextReachesHistoryThroughTheChangeEvent(t *testing.T) {
	set := fake.NewSet()
	ports := set.Ports()
	history := &clipboardHistory{clipboard: ports.Clipboard}

	set.Clipboard.WriteText("first copy")
	if !history.observeChange(t) {
		t.Fatal("text copy was not recorded")
	}

	set.Clipboard.WriteImagePNG(iconPNG("screenshot", 16))
	if !history.observeChange(t) || history.images != 1 {
		t.Fatalf("image copy was not recorded: images %d", history.images)
	}

	set.Clipboard.WriteConcealedText("hunter2-password")
	if history.observeChange(t) {
		t.Fatal("concealed copy was recorded")
	}

	set.Clipboard.WriteText("second copy")
	if !history.observeChange(t) {
		t.Fatal("second text copy was not recorded")
	}
	if got := history.text; len(got) != 2 || got[0] != "first copy" || got[1] != "second copy" {
		t.Fatalf("history text = %q", got)
	}
}

// TestClipboardFixtureSequenceNeverRecordsConcealedEntries replays generated clipboard history through the port
// and checks that the history holds exactly the entries that should be recorded.
func TestClipboardFixtureSequenceNeverRecordsConcealedEntries(t *testing.T) {
	set := fake.NewSet()
	history := &clipboardHistory{clipboard: set.Ports().Clipboard}
	entries := ClipboardEntries(60)

	var wantText []string
	var wantImages int
	for _, entry := range entries {
		switch {
		case entry.Concealed:
			set.Clipboard.WriteConcealedText(entry.Text)
		case entry.PNG != nil:
			wantImages++
			set.Clipboard.WriteImagePNG(entry.PNG)
		default:
			wantText = append(wantText, entry.Text)
			set.Clipboard.WriteText(entry.Text)
		}
		history.observeChange(t)
	}

	if len(history.text) != len(wantText) || history.images != wantImages {
		t.Fatalf("recorded %d text and %d images, want %d and %d", len(history.text), history.images, len(wantText), wantImages)
	}
	for i, text := range history.text {
		if text != wantText[i] {
			t.Fatalf("history[%d] = %q, want %q", i, text, wantText[i])
		}
	}
}

// TestSeededSetInstallsFixtureAppsWithIcons checks the shell fixtures are reachable through the port.
func TestSeededSetInstallsFixtureAppsWithIcons(t *testing.T) {
	ports := SeededSet().Ports()
	apps, err := ports.Shell.InstalledApps()
	if err != nil || len(apps) != DefaultAppCount {
		t.Fatalf("InstalledApps = %d apps, %v", len(apps), err)
	}
	icon, err := ports.Shell.AppIcon(apps[0].Path, 64)
	if err != nil || len(icon) == 0 {
		t.Fatalf("AppIcon of first app: %d bytes, %v", len(icon), err)
	}
}
