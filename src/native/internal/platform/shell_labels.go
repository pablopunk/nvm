package platform

// ShellLabels are the titles of OS actions, chosen so each reads the way the user's system names them.
type ShellLabels struct {
	RevealInFileManager string
	PreviewFile         string
	OpenSystemSettings  string
	KeyboardSettings    string
	// SettingsCommand titles the command that opens the system settings app, which is the OS's own name for it.
	SettingsCommand string
}

// ShellLabelsFor returns the titles for the operating system goos (runtime.GOOS names).
func ShellLabelsFor(goos string) ShellLabels {
	switch goos {
	case "darwin":
		return ShellLabels{RevealInFileManager: "Reveal in Finder", PreviewFile: "Quick Look", OpenSystemSettings: "Open System Settings", KeyboardSettings: "Keyboard Settings", SettingsCommand: "Open System Settings"}
	case "windows":
		return ShellLabels{RevealInFileManager: "Show in Explorer", PreviewFile: "Preview", OpenSystemSettings: "Open Settings", KeyboardSettings: "Keyboard Settings", SettingsCommand: "Open Settings"}
	}
	return ShellLabels{RevealInFileManager: "Show in Files", PreviewFile: "Preview", OpenSystemSettings: "Open System Settings", KeyboardSettings: "Keyboard Settings", SettingsCommand: "Open Settings"}
}
