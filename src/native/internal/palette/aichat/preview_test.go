package aichat

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
)

// previewRigWith gives the rig a preview source whose rows the test can change, as an extension's rows change after a
// write.
func previewRigWith(t *testing.T, rows *[]PreviewRow) *rig {
	t.Helper()
	r := newRig(t, nil)
	r.previewRows = func(string) []PreviewRow { return *rows }
	return r
}

// TestBuilderPreviewShowsTheExtensionRowsBesideTheChat checks a builder chat bound to an extension offers Show Preview,
// and opening it shows the extension's rows beside the transcript.
func TestBuilderPreviewShowsTheExtensionRowsBesideTheChat(t *testing.T) {
	rows := []PreviewRow{{Title: "Local time", Subtitle: "Clock"}}
	r := previewRigWith(t, &rows)
	record := r.builderWith("user.clock", user("A clock."), assistant("Installed."))
	view := ChatView(r.deps(), record.ID)
	if !contains(menuTitles(view), "Show Preview") {
		t.Fatalf("menu = %v, want Show Preview for a builder chat with an extension", menuTitles(view))
	}
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	if tt.HasText("Local time") {
		t.Fatal("the preview is shown before it is opened")
	}
	chooseMenuAction(t, r, tt, view, "Show Preview")
	tt.Frame()
	if !tt.HasText("Local time") || !tt.HasText("Preview · user.clock") {
		t.Fatal("Show Preview did not show the extension's rows")
	}
}

// TestBuilderPreviewFollowsTheExtensionAfterEachWrite checks the preview shows the rows the extension has now: a change
// to the rows shows on the next frame.
func TestBuilderPreviewFollowsTheExtensionAfterEachWrite(t *testing.T) {
	rows := []PreviewRow{{Title: "Local time"}}
	r := previewRigWith(t, &rows)
	record := r.builderWith("user.clock", user("A clock."), assistant("Installed."))
	view := ChatView(r.deps(), record.ID)
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	chooseMenuAction(t, r, tt, view, "Show Preview")
	rows = []PreviewRow{{Title: "Local time"}, {Title: "Date"}}
	tt.Frame()
	if !tt.HasText("Date") {
		t.Fatal("the preview did not pick up the extension's new row")
	}
}

// TestConversationHasNoPreview checks a conversation, which has no extension, offers no preview.
func TestConversationHasNoPreview(t *testing.T) {
	rows := []PreviewRow{{Title: "Local time"}}
	r := previewRigWith(t, &rows)
	record := r.chatWith("Plan", chat.TierFast, user("q"), assistant("a"))
	if contains(menuTitles(ChatView(r.deps(), record.ID)), "Show Preview") {
		t.Fatal("a conversation should not offer a preview")
	}
}
