package platform

// ShellLabels are the titles of OS actions, chosen so each reads the way the user's system names them.
type ShellLabels struct {
	RevealInFileManager string
	PreviewFile         string
	OpenSystemSettings  string
	KeyboardSettings    string
}

// SettingsTitleFor returns the title of the command that opens the system settings on goos (runtime.GOOS names).
func SettingsTitleFor(goos string) string {
	if goos == "darwin" {
		return "Open System Settings"
	}
	return "Open Settings"
}

// ShellLabelsFor returns the titles for the operating system goos (runtime.GOOS names).
func ShellLabelsFor(goos string) ShellLabels {
	switch goos {
	case "darwin":
		return ShellLabels{RevealInFileManager: "Reveal in Finder", PreviewFile: "Quick Look", OpenSystemSettings: "Open System Settings", KeyboardSettings: "Keyboard Settings"}
	case "windows":
		return ShellLabels{RevealInFileManager: "Show in Explorer", PreviewFile: "Preview", OpenSystemSettings: "Open Settings", KeyboardSettings: "Keyboard Settings"}
	}
	return ShellLabels{RevealInFileManager: "Show in Files", PreviewFile: "Preview", OpenSystemSettings: "Open System Settings", KeyboardSettings: "Keyboard Settings"}
}
