package aicommands

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Indicator timings: a result stays for the time it takes to read, a failure for a second line.
const (
	successDismissMs = 2000
	fixDismissMs     = 4000
	undoDismissMs    = 2000
	failureDismissMs = 4000
)

// errPasteUnconfirmed is the failure of a replacement the host could not confirm.
var errPasteUnconfirmed = errorString("Could not paste the fix. The text is on the clipboard")

// errorString is a failure whose text is its copy.
type errorString string

func (e errorString) Error() string { return string(e) }

// hud draws the command's indicator: progress while it works, then the result or the failure.
type hud struct {
	ctx   api.Context
	title string
}

// newHUD returns the indicator of the command titled title.
func newHUD(ctx api.Context, title string) hud {
	return hud{ctx: ctx, title: title}
}

// show sets the indicator to a loading subtitle.
func (h hud) show(subtitle string) {
	_ = h.ctx.Indicators().Show(api.IndicatorInput{ID: "ai-commands", Title: h.title, Subtitle: subtitle, Status: api.IndicatorLoading})
}

// progress shows the piece count of a correction as the indicator's bar.
func (h hud) progress(done, total int) {
	_ = h.ctx.Indicators().Update(api.IndicatorInput{ID: "ai-commands", Title: h.title, Subtitle: "Fixing Text", Status: api.IndicatorLoading, Value: done, Total: total})
}

// succeed shows subtitle as a success for durationMs and returns nil.
func (h hud) succeed(subtitle string, durationMs int64) error {
	_ = h.ctx.Indicators().Update(api.IndicatorInput{ID: "ai-commands", Title: h.title, Subtitle: subtitle, Status: api.IndicatorSuccess, DurationMs: durationMs})
	return nil
}

// fail shows err's copy as an error for a while and returns err.
func (h hud) fail(err error) error {
	_ = h.ctx.Indicators().Update(api.IndicatorInput{ID: "ai-commands", Title: h.title, Subtitle: err.Error(), Status: api.IndicatorError, DurationMs: failureDismissMs})
	return err
}
