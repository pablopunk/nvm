package system

import "regexp"

// Pane is one macOS System Settings pane the palette can open directly.
type Pane struct {
	ID    string
	Title string
}

// knownPanes are the panes the app knows how to open, in the order of the Settings sidebar on current macOS.
// A pane the sidebar of this macOS does not list is left out, so the root never offers a pane that is missing.
var knownPanes = []Pane{
	{ID: "com.apple.systempreferences.AppleIDSettings", Title: "Apple Account"},
	{ID: "com.apple.Family-Settings.extension", Title: "Family"},
	{ID: "com.apple.wifi-settings-extension", Title: "Wi-Fi"},
	{ID: "com.apple.BluetoothSettings", Title: "Bluetooth"},
	{ID: "com.apple.Network-Settings.extension", Title: "Network"},
	{ID: "com.apple.Notifications-Settings.extension", Title: "Notifications"},
	{ID: "com.apple.Sound-Settings.extension", Title: "Sound"},
	{ID: "com.apple.Focus-Settings.extension", Title: "Focus"},
	{ID: "com.apple.Screen-Time-Settings.extension", Title: "Screen Time"},
	{ID: "com.apple.systempreferences.GeneralSettings", Title: "General"},
	{ID: "com.apple.Appearance-Settings.extension", Title: "Appearance"},
	{ID: "com.apple.Accessibility-Settings.extension", Title: "Accessibility"},
	{ID: "com.apple.ControlCenter-Settings.extension", Title: "Control Center"},
	{ID: "com.apple.Siri-Settings.extension", Title: "Apple Intelligence & Siri"},
	{ID: "com.apple.Spotlight-Settings.extension", Title: "Spotlight"},
	{ID: "com.apple.settings.PrivacySecurity.extension", Title: "Privacy & Security"},
	{ID: "com.apple.Desktop-Settings.extension", Title: "Desktop & Dock"},
	{ID: "com.apple.Displays-Settings.extension", Title: "Displays"},
	{ID: "com.apple.Wallpaper-Settings.extension", Title: "Wallpaper"},
	{ID: "com.apple.Battery-Settings.extension", Title: "Battery"},
	{ID: "com.apple.Lock-Screen-Settings.extension", Title: "Lock Screen"},
	{ID: "com.apple.Touch-ID-Settings.extension", Title: "Touch ID & Password"},
	{ID: "com.apple.Users-Groups-Settings.extension", Title: "Users & Groups"},
	{ID: "com.apple.Passwords-Settings.extension", Title: "Passwords"},
	{ID: "com.apple.Internet-Accounts-Settings.extension", Title: "Internet Accounts"},
	{ID: "com.apple.Game-Center-Settings.extension", Title: "Game Center"},
	{ID: "com.apple.WalletSettingsExtension", Title: "Wallet & Apple Pay"},
	{ID: "com.apple.Keyboard-Settings.extension", Title: "Keyboard"},
	{ID: "com.apple.Mouse-Settings.extension", Title: "Mouse"},
	{ID: "com.apple.Trackpad-Settings.extension", Title: "Trackpad"},
	{ID: "com.apple.Game-Controller-Settings.extension", Title: "Game Controllers"},
	{ID: "com.apple.CD-DVD-Settings.extension", Title: "CDs & DVDs"},
	{ID: "com.apple.Print-Scan-Settings.extension", Title: "Printers & Scanners"},
}

// sidebarPaneID finds the pane identifiers a sidebar property list names. Each is a string element of the file.
var sidebarPaneID = regexp.MustCompile(`<string>([^<]+)</string>`)

// PanesFromSidebar returns the known panes that the Settings sidebar lists, in the sidebar's order, each once.
// A sidebar that cannot be read yields no panes; the caller then offers none.
func PanesFromSidebar(sidebar string) []Pane {
	known := make(map[string]Pane, len(knownPanes))
	for _, pane := range knownPanes {
		known[pane.ID] = pane
	}
	var panes []Pane
	seen := map[string]bool{}
	for _, match := range sidebarPaneID.FindAllStringSubmatch(sidebar, -1) {
		pane, ok := known[match[1]]
		if !ok || seen[pane.ID] {
			continue
		}
		seen[pane.ID] = true
		panes = append(panes, pane)
	}
	return panes
}
