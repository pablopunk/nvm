package shortcuts

// DefaultPalette is the palette hotkey of a new installation.
const DefaultPalette = "Alt+Shift+Space"

// linuxRecoveryPalettes are bound, in order, when the palette hotkey is taken on Linux, where desktops may hold keys
// that the app asks for. The palette stays reachable through whichever one binds.
var linuxRecoveryPalettes = []string{"Control+Alt+K", "Control+Alt+Shift+K", "Control+Shift+Space"}

// SystemReserved names the system feature that already owns accelerator on goos, or "" when none does. Spotlight owns
// Command+Space on macOS, so a palette hotkey there is flagged rather than refused.
func SystemReserved(accelerator, goos string) string {
	if goos == "darwin" && Normalize(accelerator) == Command+"+Space" {
		return "Spotlight"
	}
	return ""
}
