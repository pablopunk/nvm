package systemcommands

import "github.com/pablopunk/nvm/native/internal/ext/api"

// command is one OS command the root finds by title or keyword. Destructive commands carry the question their
// confirmation asks; the host runs the op once the user confirms.
type command struct {
	op          string
	title       string
	subtitle    string
	keywords    string
	icon        string
	tone        api.Color
	capability  string
	destructive bool
	confirm     string
}

// catalog lists the commands in the order the palette lists them. The settings command takes the OS's own name for
// its settings app, which is the only title that differs by OS.
func catalog(settingsCommand string) []command {
	return []command{
		{op: "lock-screen", title: "Lock Screen", subtitle: "Secure this computer", keywords: "lock screen security", icon: "lock", capability: "system.lock"},
		{op: "sleep", title: "Sleep", subtitle: "Put this computer to sleep", keywords: "sleep suspend", icon: "moon", capability: "system.sleep"},
		{op: "log-out", title: "Log Out", subtitle: "End the current session", keywords: "logout sign out", icon: "log-out", capability: "system.logOut",
			destructive: true, confirm: "End the current session. Unsaved work in open apps may be lost."},
		{op: "restart", title: "Restart Computer", subtitle: "Restart this computer", keywords: "reboot restart", icon: "rotate-ccw", tone: api.ColorOrange, capability: "system.restart",
			destructive: true, confirm: "Restart this computer. Unsaved work in open apps may be lost."},
		{op: "shut-down", title: "Shut Down", subtitle: "Power off this computer", keywords: "shutdown power off", icon: "power", tone: api.ColorRed, capability: "system.shutDown",
			destructive: true, confirm: "Power off this computer. Unsaved work in open apps may be lost."},
		{op: "empty-trash", title: "Empty Trash", subtitle: "Permanently delete trashed files", keywords: "trash delete bin empty", icon: "trash-2", tone: api.ColorRed, capability: "system.emptyTrash",
			destructive: true, confirm: "Permanently delete the files in the Trash. This cannot be undone."},
		{op: "open-system-settings", title: settingsCommand, subtitle: "Open system preferences", keywords: "settings preferences system", icon: "settings", capability: "system.settings"},
	}
}

// Titles maps the op of each system command to its title, with settingsCommand as the System Settings title.
func Titles(settingsCommand string) map[string]string {
	titles := map[string]string{}
	for _, c := range catalog(settingsCommand) {
		titles[c.op] = c.title
	}
	return titles
}

// SupportedCount is how many of the commands the system supports, given whether it has a capability by name.
func SupportedCount(hasCapability func(name string) bool) int {
	count := 0
	for _, c := range catalog("") {
		if hasCapability(c.capability) {
			count++
		}
	}
	return count
}
