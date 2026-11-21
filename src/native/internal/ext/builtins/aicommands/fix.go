package aicommands

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fix corrects the selected text of the source app and replaces it in place. It reads the selection, sends it to the
// fast model piece by piece, and replaces the selection only when every piece passed the guard. The indicator shows
// progress and the outcome.
func (e Extension) fix(ctx api.Context) error {
	hud := newHUD(ctx, FixTitle)
	hud.show("Reading Selection")
	text, found, err := ctx.Desktop().Selection().Text()
	if err != nil {
		return hud.fail(err)
	}
	if !found || strings.TrimSpace(text) == "" {
		return hud.fail(errNoSelection)
	}
	ai := ctx.AI()
	available, err := ai.IsAvailable()
	if err != nil {
		return hud.fail(err)
	}
	if !available {
		return hud.fail(errNotSignedIn)
	}
	_ = ai.Prepare(api.AIOptions{Model: api.ModelFast, System: commands.FixSystemPrompt})
	hud.show("Fixing Text")
	corrected, err := commands.CorrectText(text, askModel(ctx), hud.progress)
	if err != nil {
		return hud.fail(err)
	}
	if corrected == text {
		return hud.succeed("No changes needed", successDismissMs)
	}
	replaced, err := ctx.Desktop().Selection().ReplaceText(corrected)
	if err != nil {
		return hud.fail(err)
	}
	if !replaced {
		return hud.fail(errPasteUnconfirmed)
	}
	e.fixes.record(ctx.Clock().Now())
	return hud.succeed("Text fixed. Undo Fix is in the palette", fixDismissMs)
}

// askModel is the one-shot model call of a correction. The system prompt and the message travel apart.
func askModel(ctx api.Context) commands.Asker {
	return func(system, user string) (string, error) {
		return ctx.AI().Fast(user, system)
	}
}
