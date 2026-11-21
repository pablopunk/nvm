package aichat

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// extensionChatFixture is the dev chat view of an extension: a finished exchange, a reply still arriving, a waiting tool
// row, the picture control on the composer and a side page beside the transcript. Its spec is the one the extension
// renderer receives, so the goldens show what an extension's chat draws.
func extensionChatFixture(t *testing.T) (ExtensionChatSpec, *palette.View) {
	t.Helper()
	side := &palette.View{Title: "Preview", Search: func(string) []search.Item {
		return []search.Item{{Title: "Summarize", Subtitle: "Command"}, {Title: "Translate", Subtitle: "Command"}}
	}}
	spec := ExtensionChatSpec{
		Title:       "Dev Chat",
		Placeholder: "Message AI",
		Tier:        chat.TierSmart,
		Attach:      true,
		PickImages:  func(func([]chat.Image)) {},
		Messages: []chat.Message{
			user("What changed in the palette this week?"),
			{Author: chat.AuthorAssistant, Reasoning: "Checking the release notes.", Text: "Search ranks apps first, and Escape stops a reply.", At: goldenNow, Status: chat.StatusComplete},
			{Author: chat.AuthorTool, Status: chat.StatusAwaiting, At: goldenNow, Tool: &chat.ToolRun{Name: "read_file", Label: "read_file", Outcome: "Allow reading notes.md?"}},
		},
		Side: func() *palette.View { return side },
	}
	return spec, side
}

// TestExtensionChatGoldens renders the extension chat view with a picture on the composer and the side page open.
func TestExtensionChatGoldens(t *testing.T) {
	r := newRig(t, nil)
	spec, _ := extensionChatFixture(t)
	x := NewExtensionChat(func(c *ui.Context, src string, _ bool) { ui.Text(c, src).FontSize(13) }, func() time.Time { return goldenNow })
	x.SetSpec(spec)
	x.images = []chat.Image{testPicture(t, "shot.png", 64, 48, 200)}
	r.open(x.View())
	golden.AssertMatrix(t, "extension-chat", r.palette.View, pageGoldens)
}

// TestExtensionChatHidesItsSideFromTheShortcut checks the preview toggle hides the side page and shows it again.
func TestExtensionChatHidesItsSideFromTheShortcut(t *testing.T) {
	spec, _ := extensionChatFixture(t)
	x := NewExtensionChat(nil, func() time.Time { return goldenNow })
	x.SetSpec(spec)
	view := x.View()
	if view.Side == nil || view.Side() == nil {
		t.Fatalf("side is hidden at first, want it shown")
	}
	host := &refreshHost{}
	findAction(t, x.actions(), "Hide Preview").Run(host)
	if host.refreshed != 1 {
		t.Fatalf("toggle refreshed the palette %d times, want once", host.refreshed)
	}
	if view.Side() != nil {
		t.Fatalf("side is still shown after Hide Preview")
	}
	if findAction(t, x.actions(), "Show Preview").Title != "Show Preview" {
		t.Fatalf("toggle did not change its title")
	}
}

// TestExtensionChatSendCarriesTextAndPictures checks a message reaches the owner with its typed text and its pictures,
// and that the composer is empty afterwards.
func TestExtensionChatSendCarriesTextAndPictures(t *testing.T) {
	var gotText string
	var gotImages []chat.Image
	spec, _ := extensionChatFixture(t)
	spec.Send = func(_ search.Host, text string, images []chat.Image) { gotText, gotImages = text, images }
	x := NewExtensionChat(nil, func() time.Time { return goldenNow })
	x.SetSpec(spec)
	x.images = []chat.Image{testPicture(t, "shot.png", 8, 8, 10)}
	x.draft = "  describe it  "
	if !x.send(x.draft) || gotText != "describe it" || len(gotImages) != 1 {
		t.Fatalf("send gave text %q and %d pictures", gotText, len(gotImages))
	}
	if len(x.images) != 0 || x.draft != "" {
		t.Fatalf("composer kept its text or pictures after send")
	}
}

// refreshHost is the host the toggle needs: it only counts the refreshes the palette is asked for.
type refreshHost struct {
	search.Host
	refreshed int
}

// Refresh counts the refresh request.
func (h *refreshHost) Refresh() { h.refreshed++ }

// findAction returns the action titled title, failing the test when there is none.
func findAction(t *testing.T, actions []search.Action, title string) search.Action {
	t.Helper()
	for _, action := range actions {
		if action.Title == title {
			return action
		}
	}
	t.Fatalf("no action %q in %d actions", title, len(actions))
	return search.Action{}
}
