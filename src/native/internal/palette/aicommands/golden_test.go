package aicommands

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// goldenWidth is the indicator panel's width; each golden takes the height Height gives its status.
const (
	goldenWidth  = 300
	paletteWidth = 720
	paletteHigh  = 470
)

// indicatorStatuses lists every status the indicator renders, in the order the goldens are written.
var indicatorStatuses = []struct {
	name   string
	status commands.Status
}{
	{"indicator-reading", commands.Status{Title: fixTitle, Subtitle: "Reading Selection", Tone: commands.ToneLoading}},
	{"indicator-fixing", commands.Status{Title: fixTitle, Subtitle: "Fixing Text", Tone: commands.ToneLoading}},
	{"indicator-no-changes", commands.Status{Title: fixTitle, Subtitle: "No changes needed", Tone: commands.ToneSuccess}},
	{"indicator-triage", commands.Status{Title: triageTitle, Subtitle: "bug report · urgency 4/5 · needs reply", Tone: commands.ToneSuccess}},
	{"indicator-failed-short", commands.Status{Title: fixTitle, Subtitle: "Sign in to use AI commands.", Tone: commands.ToneError}},
	{"indicator-failed-long", commands.Status{Title: "Classify Image with AI", Subtitle: "That image is too large to classify. Choose a smaller one and try again from the palette", Tone: commands.ToneError}},
}

// TestIndicatorGoldens renders every indicator status in light and dark at 1x and 2x, at the height the panel gives it.
func TestIndicatorGoldens(t *testing.T) {
	for _, entry := range indicatorStatuses {
		t.Run(entry.name, func(t *testing.T) {
			status := entry.status
			options := golden.Options{Group: "aicommands", Width: goldenWidth, Height: Height(status)}
			golden.AssertMatrix(t, entry.name, func(c *ui.Context) { Indicator(c, status) }, options)
		})
	}
}

// fixTitle and triageTitle are the titles of the AI commands, which the AI commands built-in owns.
const (
	fixTitle    = "Fix Selected Text with AI"
	triageTitle = "Triage Selected Text with AI"
)

// testWindow is the palette's window in the goldens: updates run at once and nothing is shown.
type testWindow struct{}

func (testWindow) Update(fn func())     { fn() }
func (testWindow) Show()                {}
func (testWindow) Hide()                {}
func (testWindow) IsVisible() bool      { return true }
func (testWindow) IsFocused() bool      { return true }
func (testWindow) Focus()               {}
func (testWindow) Size() (int, int)     { return paletteWidth, paletteHigh }
func (testWindow) SetPosition(x, y int) {}

// paletteOver returns a palette with view pushed above an empty root, drawn synchronously.
func paletteOver(view *palette.View) *palette.Palette {
	root := &palette.View{Title: "Nevermind", Search: func(string) []search.Item { return nil }}
	p := palette.New(palette.Options{Root: root, Clock: fake.NewClock(), Synchronous: true})
	p.Attach(testWindow{})
	p.SetStack(root, view)
	p.SetQuery("")
	return p
}
