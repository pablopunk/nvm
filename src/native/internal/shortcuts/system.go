package shortcuts

// DefaultPalette is the palette hotkey of a new installation.
const DefaultPalette = "Alt+Shift+Space"

// SystemReserved names the system feature that already owns accelerator on goos, or "" when none does. Spotlight owns
// Command+Space on macOS, so a palette hotkey there is flagged rather than refused.
func SystemReserved(accelerator, goos string) string {
	if goos == "darwin" && Normalize(accelerator) == Command+"+Space" {
		return "Spotlight"
	}
	return ""
}
