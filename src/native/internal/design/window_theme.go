package design

import "github.com/egoist/mygo/ui"

// WindowTheme maps one appearance's tokens onto MyGo's theme, so the controls MyGo draws (text areas, text inputs,
// focus rings, selections, scrollbars) take the paper, ink and gold of Nevermind instead of MyGo's stock colors.
// Wrap a window's content in it with ui.Themed; its Background stays the caller's to choose.
func WindowTheme(dark bool) *ui.Theme {
	base := ui.LightTheme()
	if dark {
		base = ui.DarkTheme()
	}
	k := TokensFor(dark)
	base.Dark = dark
	base.Background = k.Bg
	base.Surface = k.Panel
	base.SurfaceHover = k.Interactive
	base.SurfacePressed = k.Selected
	base.Border = k.Border
	base.Text = k.Primary
	base.TextMuted = k.Muted
	base.Accent = k.Accent
	base.AccentHover = k.AccentFill
	base.AccentPressed = k.AccentEmphasis
	base.AccentText = k.AccentContrast
	base.Danger = k.Danger
	base.Warning = k.Tones["orange"]
	base.Success = k.Tones["green"]
	base.Selection = k.AccentMuted
	base.Focus = k.AccentEmphasis
	base.Scrollbar = k.EdgeStrong
	return base
}
