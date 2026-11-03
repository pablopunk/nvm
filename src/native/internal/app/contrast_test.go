package app

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// knownFailures lists text-on-surface pairs that fail WCAG AA with the prototype's tokens today. The
// test passes so the debt is visible without being hidden; the design owner decides whether the tokens
// change or the pair stays. A waiver whose pair starts passing fails the test, so it must be removed.
//
// Measured with the tokens of src/app/palette/styles/theme.css as ported to theme.go, before WP0.8: the
// muted and tertiary text and the tone colors fall below 4.5:1 on the selected plate, and the two faintest
// texts also fall below it on the interactive fill in light.
var knownFailures = map[string]knownFailure{
	"light tertiary on interactive": {ratio: 4.37, why: "tertiary text on the light control fill"},
	"light muted on interactive":    {ratio: 4.00, why: "muted text on the light control fill"},
	"light tertiary on selected":    {ratio: 3.90, why: "tertiary text on the light selected plate"},
	"light muted on selected":       {ratio: 3.57, why: "muted text on the light selected plate"},
	"light green tone on selected":  {ratio: 4.41, why: "green tone on the light selected plate"},
	"light orange tone on selected": {ratio: 4.49, why: "orange tone on the light selected plate"},
	"light pink tone on selected":   {ratio: 4.34, why: "pink tone on the light selected plate"},
	"light red tone on selected":    {ratio: 4.48, why: "red tone on the light selected plate"},
	"light yellow tone on selected": {ratio: 4.47, why: "yellow tone on the light selected plate"},
	"dark tertiary on selected":     {ratio: 4.44, why: "tertiary text on the dark selected plate"},
	"dark muted on selected":        {ratio: 3.79, why: "muted text on the dark selected plate"},
}

type knownFailure struct {
	ratio float64 // the measured ratio when the waiver was recorded
	why   string
}

type contrastPair struct {
	name   string
	fg, bg ui.Color
}

// TestTokenTextPairsMeetWCAGAA checks every text color the palette draws on a surface it draws on, in both
// appearances, against WCAG 2.1 AA for normal text. The tokens are read through tk, the same path the
// renderer uses.
func TestTokenTextPairsMeetWCAGAA(t *testing.T) {
	seen := map[string]bool{}
	for _, mode := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		rendered := tokensAsRendered(t, mode.dark)
		for _, pair := range textOnSurfacePairs(mode.name, rendered) {
			seen[pair.name] = true
			t.Run(pair.name, func(t *testing.T) {
				checkPairAgainstWaivers(t, pair)
			})
		}
	}
	for name := range knownFailures {
		if !seen[name] {
			t.Errorf("knownFailures entry %q matches no text-on-surface pair", name)
		}
	}
}

func checkPairAgainstWaivers(t *testing.T, pair contrastPair) {
	t.Helper()
	fg, bg := toDesignColor(pair.fg), toDesignColor(pair.bg)
	ratio := design.Contrast(fg, bg)
	passes := design.AA(fg, bg, false)
	waiver, waived := knownFailures[pair.name]
	switch {
	case passes && waived:
		t.Errorf("%s now meets AA at %.2f:1; remove its knownFailures entry", pair.name, ratio)
	case !passes && !waived:
		t.Errorf("%s fails AA at %.2f:1; WCAG AA for normal text needs 4.50:1", pair.name, ratio)
	case !passes && math.Abs(ratio-waiver.ratio) > 0.01:
		t.Errorf("%s measures %.2f:1 but its knownFailures entry records %.2f; update the entry", pair.name, ratio, waiver.ratio)
	case !passes:
		t.Logf("known failure %s: %.2f:1 (%s)", pair.name, ratio, waiver.why)
	}
}

// tokensAsRendered returns the tokens the palette renders with in the requested appearance. It renders a
// tester view and reads tk from it, so the test follows the same path as the renderer.
func tokensAsRendered(t *testing.T, dark bool) *tokens {
	t.Helper()
	var rendered *tokens
	tt := ui.NewTester(func(c *ui.Context) { rendered = tk(c) }, 64, 64)
	tt.SetDark(dark)
	tt.Frame()
	if rendered == nil {
		t.Fatal("the palette view did not render any tokens")
	}
	want := lightTokens
	if dark {
		want = darkTokens
	}
	if rendered != want {
		t.Fatalf("tk returned the wrong appearance for dark=%v", dark)
	}
	return rendered
}

// textOnSurfacePairs lists the text colors the palette uses and the surfaces each one sits on.
func textOnSurfacePairs(mode string, k *tokens) []contrastPair {
	surfaces := []struct {
		name  string
		color ui.Color
	}{{"panel", k.panel}, {"raised", k.raised}, {"interactive", k.interactive}, {"selected", k.selected}}
	texts := []struct {
		name  string
		color ui.Color
	}{{"strong", k.strong}, {"primary", k.primary}, {"secondary", k.secondary}, {"tertiary", k.tertiary}, {"muted", k.muted}}

	var pairs []contrastPair
	for _, surface := range surfaces {
		for _, text := range texts {
			pairs = append(pairs, contrastPair{
				name: fmt.Sprintf("%s %s on %s", mode, text.name, surface.name),
				fg:   text.color,
				bg:   surface.color,
			})
		}
	}
	pairs = append(pairs, contrastPair{name: mode + " accent text on accentFill", fg: k.accentText, bg: k.accentFill})
	for _, tone := range sortedToneNames(k) {
		for _, surface := range []struct {
			name  string
			color ui.Color
		}{{"panel", k.panel}, {"selected", k.selected}} {
			pairs = append(pairs, contrastPair{
				name: fmt.Sprintf("%s %s tone on %s", mode, tone, surface.name),
				fg:   k.tones[tone],
				bg:   surface.color,
			})
		}
	}
	pairs = append(pairs, contrastPair{name: mode + " danger on panel", fg: k.danger, bg: k.panel})
	return pairs
}

func sortedToneNames(k *tokens) []string {
	names := make([]string, 0, len(k.tones))
	for name := range k.tones {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func toDesignColor(c ui.Color) design.Color {
	return design.Color{R: c.R, G: c.G, B: c.B, A: c.A}
}
