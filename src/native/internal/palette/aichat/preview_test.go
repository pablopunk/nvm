package aichat

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// previewRoot is the first page of an extension's preview: its rows as they are now, read on every search.
func previewRoot(titles *[]string) *palette.View {
	return &palette.View{Title: "user.clock", HideInput: true, Search: func(string) []search.Item {
		var out []search.Item
		for _, title := range *titles {
			out = append(out, search.Item{Kind: "Clock", Title: title, Subtitle: "Clock"})
		}
		return out
	}}
}

// previewRigWith gives the rig an extension preview whose rows the test can change, as an extension's rows change after
// a write.
func previewRigWith(t *testing.T, titles *[]string) (*rig, *palette.Side) {
	t.Helper()
	r := newRig(t, nil)
	side := palette.NewSide(previewRoot(titles))
	r.previewSide = side
	return r, side
}

// TestBuilderPreviewShowsTheExtensionBesideTheChat checks a builder chat bound to an extension shows its preview at
// once, as 1.x does after a write, and Hide Preview closes it and Show Preview opens it again.
func TestBuilderPreviewShowsTheExtensionBesideTheChat(t *testing.T) {
	titles := []string{"Local time"}
	r, _ := previewRigWith(t, &titles)
	record := r.builderWith("user.clock", user("A clock."), assistant("Installed."))
	view := ChatView(r.deps(), record.ID)
	if !contains(menuTitles(view), "Hide Preview") {
		t.Fatalf("menu = %v, want Hide Preview for a builder chat with an extension", menuTitles(view))
	}
	r.open(view)
	tt := ui.NewTester(r.palette.View, palette.SplitWidth, palette.SplitHeight)
	tt.Frame()
	if !tt.HasText("Local time") {
		t.Fatal("the preview is not shown beside the chat")
	}
	chooseMenuAction(t, r, tt, view, "Hide Preview")
	tt.Frame()
	if tt.HasText("Local time") {
		t.Fatal("Hide Preview did not close the preview")
	}
	if !contains(menuTitles(view), "Show Preview") {
		t.Fatalf("menu = %v, want Show Preview once hidden", menuTitles(view))
	}
}

// TestBuilderPreviewFollowsTheExtensionAfterEachWrite checks a refresh of the extension's side shows the rows it has
// now: a new row shows on the next frame.
func TestBuilderPreviewFollowsTheExtensionAfterEachWrite(t *testing.T) {
	titles := []string{"Local time"}
	r, side := previewRigWith(t, &titles)
	record := r.builderWith("user.clock", user("A clock."), assistant("Installed."))
	view := ChatView(r.deps(), record.ID)
	r.open(view)
	tt := ui.NewTester(r.palette.View, palette.SplitWidth, palette.SplitHeight)
	tt.Frame()
	titles = []string{"Local time", "Date"}
	side.Refresh()
	tt.Frame()
	if !tt.HasText("Date") {
		t.Fatal("the preview did not pick up the extension's new row")
	}
}

// TestFocusPreviewGivesTheKeyboardToThePreviewAndEscapeGivesItBack checks Focus Preview moves the arrows to the
// preview's rows, and Escape in the preview gives the keyboard back to the chat without closing it.
func TestFocusPreviewGivesTheKeyboardToThePreviewAndEscapeGivesItBack(t *testing.T) {
	titles := []string{"Local time", "Date"}
	r, side := previewRigWith(t, &titles)
	record := r.builderWith("user.clock", user("A clock."), assistant("Installed."))
	view := ChatView(r.deps(), record.ID)
	r.open(view)
	tt := ui.NewTester(r.palette.View, palette.SplitWidth, palette.SplitHeight)
	tt.Frame()
	chooseMenuAction(t, r, tt, view, "Focus Preview")
	if !side.Focused() {
		t.Fatal("Focus Preview did not give the preview the keyboard")
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if side.Focused() {
		t.Fatal("Escape did not give the keyboard back to the chat")
	}
	if !r.palette.Visible() {
		t.Fatal("Escape in the preview hid the palette")
	}
}

// TestConversationHasNoPreview checks a conversation, which has no extension, offers no preview.
func TestConversationHasNoPreview(t *testing.T) {
	titles := []string{"Local time"}
	r, _ := previewRigWith(t, &titles)
	record := r.chatWith("Plan", chat.TierFast, user("q"), assistant("a"))
	if contains(menuTitles(ChatView(r.deps(), record.ID)), "Show Preview") || contains(menuTitles(ChatView(r.deps(), record.ID)), "Hide Preview") {
		t.Fatal("a conversation should not offer a preview")
	}
}
