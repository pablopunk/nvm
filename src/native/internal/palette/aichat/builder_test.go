package aichat

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// builderWith seeds a builder chat bound to extension, with the given transcript.
func (r *rig) builderWith(extension string, messages ...chat.Message) chat.Chat {
	r.t.Helper()
	record, err := r.service.NewChat(chat.KindBuilder, chat.TierSmart)
	if err != nil {
		r.t.Fatalf("NewChat: %v", err)
	}
	if err := r.store.Mutate(record.ID, goldenNow, func(c *chat.Chat) {
		c.Extension = extension
		c.Messages = messages
	}); err != nil {
		r.t.Fatalf("seed builder chat: %v", err)
	}
	r.service.Reload()
	record, _ = r.service.Chat(record.ID)
	return record
}

// installedNotice is the note a builder writes after it installs an extension enabled.
func installedNotice() chat.Notice {
	return chat.Notice{Kind: chat.NoticeNote, Title: "Extension", Message: "“Clock” is installed and enabled."}
}

// TestReviewKeyOpensTheExtensionReview checks ⌘E pushes the review of the extension the builder chat wrote, above the
// chat, so Escape returns to the chat.
func TestReviewKeyOpensTheExtensionReview(t *testing.T) {
	r := newRig(t, nil)
	review := &palette.View{Title: "Review Clock"}
	deps := r.deps()
	deps.Review = func(id string) *palette.View {
		if id != "user.clock" {
			t.Fatalf("reviewed %q; want user.clock", id)
		}
		return review
	}
	record := r.builderWith("user.clock", user("Make a clock."), assistant("Done."), notice(installedNotice()))
	view := ChatView(deps, record.ID)
	r.palette.Show()
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Key(palette.Mod, ui.KeyE)

	if r.palette.Top() != review {
		t.Fatal("⌘E did not open the extension review")
	}
}

// TestReviewKeyWithoutAnExtensionDoesNothing: a conversation, or a builder chat with no extension yet, has nothing to
// review.
func TestReviewKeyWithoutAnExtensionDoesNothing(t *testing.T) {
	r := newRig(t, nil)
	deps := r.deps()
	deps.Review = func(string) *palette.View {
		t.Fatal("a chat without an extension asked for a review")
		return nil
	}
	record := r.builderWith("", user("Make a clock."))
	view := ChatView(deps, record.ID)
	r.palette.Show()
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Key(palette.Mod, ui.KeyE)

	if r.palette.Top() != view {
		t.Fatal("⌘E moved away from a chat that has no extension to review")
	}
}

// TestBuilderInstalledGoldens renders a builder chat that installed an extension enabled: the review hint in the footer
// and the note that says it is on.
func TestBuilderInstalledGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.builderWith("user.clock",
		user("Make a clock that shows the local time."),
		assistant("I wrote Clock. It shows the local time in the search results."),
		notice(installedNotice()))
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "builder-installed", r.palette.View, pageGoldens)
}

// TestBuilderFailedGoldens renders a builder chat that could not produce a working extension.
func TestBuilderFailedGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.builderWith("",
		user("Make a clock that shows the local time."),
		notice(chat.Notice{Kind: chat.NoticeFailed, Title: "Extension not installed",
			Message: "Could not produce a working extension: extension source does not compile: undefined: api.Itme. Describe it again to retry."}))
	r.open(ChatView(r.deps(), record.ID))
	golden.AssertMatrix(t, "builder-failed", r.palette.View, pageGoldens)
}
