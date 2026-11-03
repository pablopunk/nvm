package app

import (
	"runtime"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

// The palette follows the system theme: colors come from ui.Theme (the system's light/dark and
// accent), icon tiles use the macOS system colors, selection is the accent color as in AppKit lists.

// mod is the shortcut modifier: ⌘ on macOS, Ctrl elsewhere.
var mod = ui.Ctrl

func init() {
	if runtime.GOOS == "darwin" {
		mod = ui.Cmd
	}
}

// kb spells a Ctrl-based label for the platform (⌘ on macOS).
func kb(label string) string {
	if runtime.GOOS == "darwin" {
		return strings.ReplaceAll(label, "⌃", "⌘")
	}
	return label
}

// tokens are Nevermind's own palette (src/app/palette/styles/theme.css): warm paper surfaces and a
// gold accent in light, ink blue-grey with the same gold in dark.
type tokens struct {
	dark                                    bool
	bg, surface, border, accent, accentText ui.Color
	strong, muted, faint                    ui.Color
	danger, hairline                        ui.Color
	panel, raised, interactive, sunken      ui.Color
	selected                                ui.Color
	edgeSubtle, edgeStrong                  ui.Color
	secondary, tertiary, primary            ui.Color
	accentMuted, accentSoft, accentFill     ui.Color
	accentEmphasis, accentContrast, shadow  ui.Color
	hi, lo, wash                            ui.Color
	tones                                   map[string]ui.Color
}

func rgba(hex string, a float32) ui.Color { return ui.Hex(hex).Alpha(a) }

var lightTokens = &tokens{
	panel: ui.Hex("#f5f2eb"), bg: ui.Hex("#f5f2eb"), raised: ui.Hex("#faf8f3"), surface: ui.Hex("#faf8f3"), interactive: ui.Hex("#e8e4dc"),
	selected: ui.Hex("#ddd8ce"), sunken: ui.Hex("#e3dfd7"),
	edgeSubtle: rgba("#363129", 0.14), edgeStrong: rgba("#363129", 0.24), border: rgba("#363129", 0.24), hairline: rgba("#363129", 0.14),
	shadow: rgba("#3e3528", 0.26),
	strong: ui.Hex("#282721"), primary: ui.Hex("#34322c"), secondary: ui.Hex("#5e5a51"), tertiary: ui.Hex("#6d685e"), muted: ui.Hex("#736e64"), faint: rgba("#736e64", 0.75),
	accent: ui.Hex("#8a6a00"), accentText: ui.Hex("#242015"), accentFill: ui.Hex("#e6bd32"), accentContrast: ui.Hex("#242015"),
	accentSoft: rgba("#8a6a00", 0.11), accentMuted: rgba("#8a6a00", 0.22), accentEmphasis: rgba("#8a6a00", 0.52),
	danger: ui.Hex("#9b4141"),
	hi:     ui.RGBA(255, 254, 250, 0.9), lo: rgba("#3e3528", 0.13), wash: rgba("#3e3528", 0.05),
	tones: map[string]ui.Color{"yellow": ui.Hex("#745d00"), "blue": ui.Hex("#285d82"), "purple": ui.Hex("#654c86"), "green": ui.Hex("#376b45"),
		"red": ui.Hex("#984646"), "orange": ui.Hex("#875325"), "pink": ui.Hex("#884f72"), "teal": ui.Hex("#285d82"), "indigo": ui.Hex("#654c86"), "gray": ui.Hex("#5e5a51")},
}

var darkTokens = &tokens{
	panel: ui.Hex("#1a1f28"), bg: ui.Hex("#1a1f28"), raised: ui.Hex("#222834"), surface: ui.Hex("#222834"), interactive: ui.Hex("#29303d"),
	selected: ui.Hex("#343d4d"), sunken: ui.Hex("#0e1117"),
	edgeSubtle: rgba("#d3e0f5", 0.10), edgeStrong: rgba("#d3e0f5", 0.18), border: rgba("#d3e0f5", 0.18), hairline: rgba("#d3e0f5", 0.10),
	shadow: ui.RGBA(0, 0, 0, 0.55),
	strong: ui.Hex("#f0f3f7"), primary: ui.Hex("#e0e5ec"), secondary: ui.Hex("#b8c0cc"), tertiary: ui.Hex("#9ca6b4"), muted: ui.Hex("#8f99a7"), faint: rgba("#8f99a7", 0.75),
	accent: ui.Hex("#f1cf54"), accentText: ui.Hex("#242015"), accentFill: ui.Hex("#e2bd39"), accentContrast: ui.Hex("#242015"),
	accentSoft: rgba("#f1cf54", 0.12), accentMuted: rgba("#f1cf54", 0.24), accentEmphasis: rgba("#f1cf54", 0.5),
	danger: ui.Hex("#e2a6a6"),
	hi:     ui.RGBA(220, 231, 249, 0.10), lo: ui.RGBA(0, 0, 0, 0.5), wash: ui.RGBA(0, 0, 0, 0.2),
	tones: map[string]ui.Color{"yellow": ui.Hex("#dfce84"), "blue": ui.Hex("#a9c9df"), "purple": ui.Hex("#c8b6df"), "green": ui.Hex("#acd1b2"),
		"red": ui.Hex("#e0aaaa"), "orange": ui.Hex("#dfbd96"), "pink": ui.Hex("#dfb2cc"), "teal": ui.Hex("#a9c9df"), "indigo": ui.Hex("#c8b6df"), "gray": ui.Hex("#b8c0cc")},
}

func tk(c *ui.Context) *tokens {
	if c.Theme().Dark {
		return darkTokens
	}
	return lightTokens
}

// ---- icons ----

var (
	iconCache = map[string]*ui.SVG{}
	iconMu    sync.Mutex
)

func svgIcon(name string) *ui.SVG {
	iconMu.Lock()
	defer iconMu.Unlock()
	if s, ok := iconCache[name]; ok {
		return s
	}
	src, ok := iconSources[name]
	if !ok {
		src = iconSources["puzzle"]
	}
	s := ui.MustParseSVG([]byte(src))
	iconCache[name] = s
	return s
}
