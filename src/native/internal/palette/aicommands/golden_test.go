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
	{"indicator-reading", commands.Status{Title: commands.FixTitle, Subtitle: "Reading Selection", Tone: commands.ToneLoading}},
	{"indicator-fixing", commands.Status{Title: commands.FixTitle, Subtitle: "Fixing Text", Tone: commands.ToneLoading}},
	{"indicator-no-changes", commands.Status{Title: commands.FixTitle, Subtitle: "No changes needed", Tone: commands.ToneSuccess}},
	{"indicator-triage", commands.Status{Title: commands.TriageTitle, Subtitle: "bug report · urgency 4/5 · needs reply", Tone: commands.ToneSuccess}},
	{"indicator-failed-short", commands.Status{Title: commands.FixTitle, Subtitle: "Sign in to use AI commands.", Tone: commands.ToneError}},
	{"indicator-failed-long", commands.Status{Title: commands.ClassifyTitle, Subtitle: "That image is too large to classify. Copy a smaller one and try again from the palette", Tone: commands.ToneError}},
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

// proofread is a short correction, the text a result page shows most often.
const proofread = "The palette got three changes this week, all behind the same shortcut. Search now ranks apps first, and Escape stops a reply in flight."

// TestResultGoldens renders a proofread that can be replaced, a copy-only classification, and a long text, each in the
// palette's page.
func TestResultGoldens(t *testing.T) {
	cases := []struct {
		name   string
		result commands.Result
	}{
		{"result-replace", commands.Result{Title: "Fixed Text", Text: func() string { return proofread }, Replace: func(string) {}, Copy: func(string) {}}},
		{"result-copy", commands.Result{Title: "Image Classification", Text: func() string { return "screenshot" }, Copy: func(string) {}}},
		{"result-long", commands.Result{Title: "Fixed Text", Text: func() string { return proofread + " " + proofread + " " + proofread }, Replace: func(string) {}, Copy: func(string) {}}},
	}
	for _, entry := range cases {
		t.Run(entry.name, func(t *testing.T) {
			p := paletteOver(ResultView(entry.result))
			golden.AssertMatrix(t, entry.name, p.View, golden.Options{Group: "aicommands", Width: paletteWidth, Height: paletteHigh})
		})
	}
}

// testWindow is the palette's window in the goldens: updates run at once and nothing is shown.
type testWindow struct{}

func (testWindow) Update(fn func())     { fn() }
func (testWindow) Show()                {}
func (testWindow) Hide()                {}
func (testWindow) IsVisible() bool      { return true }
func (testWindow) IsFocused() bool      { return true }
func (testWindow) Focus() {}
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
