package design

import (
	"github.com/egoist/mygo/ui"
)

// TokenSet is one appearance of Nevermind's palette, ported from src/app/palette/styles/theme.css: warm
// paper surfaces and a gold accent in light, ink blue-grey with the same gold in dark. Feature code reads
// colors only from a TokenSet, never from a literal.
type TokenSet struct {
	Bg, Surface, Border, Accent, AccentText ui.Color
	Strong, Muted, Faint                    ui.Color
	Danger, Hairline                        ui.Color
	Panel, Raised, Interactive, Sunken      ui.Color
	Canvas, Link                            ui.Color
	Selected                                ui.Color
	EdgeSubtle, EdgeStrong                  ui.Color
	Secondary, Tertiary, Primary            ui.Color
	AccentMuted, AccentSoft, AccentFill     ui.Color
	AccentEmphasis, AccentContrast, Shadow  ui.Color
	Hi, Lo, Wash                            ui.Color
	// Tones are the colors of the tinted titles and tiles, by tone name (yellow, blue, ...).
	Tones map[string]ui.Color
}

// Tokens returns the token set of the appearance c renders in: the dark one when the theme is dark.
func Tokens(c *ui.Context) *TokenSet {
	return TokensFor(c.Theme().Dark)
}

// TokensFor returns the light or the dark token set. Golden and contrast tests use it to name a set without
// a render.
func TokensFor(dark bool) *TokenSet {
	if dark {
		return darkTokens
	}
	return lightTokens
}

func rgba(hex string, a float32) ui.Color { return ui.Hex(hex).Alpha(a) }

var (
	lightTokens = &TokenSet{
		Canvas: ui.Hex("#ece9e2"), Link: ui.Hex("#745d00"),
		Panel: ui.Hex("#f5f2eb"), Bg: ui.Hex("#f5f2eb"), Raised: ui.Hex("#faf8f3"), Surface: ui.Hex("#faf8f3"), Interactive: ui.Hex("#e8e4dc"),
		Selected: ui.Hex("#ddd8ce"), Sunken: ui.Hex("#e3dfd7"),
		EdgeSubtle: rgba("#363129", 0.14), EdgeStrong: rgba("#363129", 0.24), Border: rgba("#363129", 0.24), Hairline: rgba("#363129", 0.14),
		Shadow: rgba("#3e3528", 0.26),
		Strong: ui.Hex("#282721"), Primary: ui.Hex("#34322c"), Secondary: ui.Hex("#5e5a51"), Tertiary: ui.Hex("#6d685e"), Muted: ui.Hex("#736e64"), Faint: rgba("#736e64", 0.75),
		Accent: ui.Hex("#8a6a00"), AccentText: ui.Hex("#242015"), AccentFill: ui.Hex("#e6bd32"), AccentContrast: ui.Hex("#242015"),
		AccentSoft: rgba("#8a6a00", 0.11), AccentMuted: rgba("#8a6a00", 0.22), AccentEmphasis: rgba("#8a6a00", 0.52),
		Danger: ui.Hex("#9b4141"),
		Hi:     ui.RGBA(255, 254, 250, 0.9), Lo: rgba("#3e3528", 0.13), Wash: rgba("#3e3528", 0.05),
		Tones: map[string]ui.Color{"yellow": ui.Hex("#745d00"), "blue": ui.Hex("#285d82"), "purple": ui.Hex("#654c86"), "green": ui.Hex("#376b45"),
			"red": ui.Hex("#984646"), "orange": ui.Hex("#875325"), "pink": ui.Hex("#884f72"), "teal": ui.Hex("#285d82"), "indigo": ui.Hex("#654c86"), "gray": ui.Hex("#5e5a51")},
	}

	darkTokens = &TokenSet{
		Canvas: ui.Hex("#12151b"), Link: ui.Hex("#d7c574"),
		Panel: ui.Hex("#1a1f28"), Bg: ui.Hex("#1a1f28"), Raised: ui.Hex("#222834"), Surface: ui.Hex("#222834"), Interactive: ui.Hex("#29303d"),
		Selected: ui.Hex("#343d4d"), Sunken: ui.Hex("#0e1117"),
		EdgeSubtle: rgba("#d3e0f5", 0.10), EdgeStrong: rgba("#d3e0f5", 0.18), Border: rgba("#d3e0f5", 0.18), Hairline: rgba("#d3e0f5", 0.10),
		Shadow: ui.RGBA(0, 0, 0, 0.55),
		Strong: ui.Hex("#f0f3f7"), Primary: ui.Hex("#e0e5ec"), Secondary: ui.Hex("#b8c0cc"), Tertiary: ui.Hex("#9ca6b4"), Muted: ui.Hex("#8f99a7"), Faint: rgba("#8f99a7", 0.75),
		Accent: ui.Hex("#f1cf54"), AccentText: ui.Hex("#242015"), AccentFill: ui.Hex("#e2bd39"), AccentContrast: ui.Hex("#242015"),
		AccentSoft: rgba("#f1cf54", 0.12), AccentMuted: rgba("#f1cf54", 0.24), AccentEmphasis: rgba("#f1cf54", 0.5),
		Danger: ui.Hex("#e2a6a6"),
		Hi:     ui.RGBA(220, 231, 249, 0.10), Lo: ui.RGBA(0, 0, 0, 0.5), Wash: ui.RGBA(0, 0, 0, 0.2),
		Tones: map[string]ui.Color{"yellow": ui.Hex("#dfce84"), "blue": ui.Hex("#a9c9df"), "purple": ui.Hex("#c8b6df"), "green": ui.Hex("#acd1b2"),
			"red": ui.Hex("#e0aaaa"), "orange": ui.Hex("#dfbd96"), "pink": ui.Hex("#dfb2cc"), "teal": ui.Hex("#a9c9df"), "indigo": ui.Hex("#c8b6df"), "gray": ui.Hex("#b8c0cc")},
	}
)
