package api

// MenuBarIcon is a curated status glyph. The list is closed so that every host renders the same
// set; unknown names fall back to MenuIconDefault and produce a diagnostic.
type MenuBarIcon string

// MenuIconDefault is the fallback glyph.
const MenuIconDefault MenuBarIcon = "zap"

// MenuBarIcons is the curated glyph set, in the same order as the TypeScript declaration.
var MenuBarIcons = []MenuBarIcon{
	"zap", "circle", "circle-dot", "circle-check", "circle-alert", "circle-x", "bell", "bell-dot",
	"timer", "clock", "alarm-clock", "calendar", "coffee", "mic", "mic-off", "video", "music",
	"play", "pause", "activity", "cpu", "hard-drive", "memory-stick", "wifi", "battery-medium",
	"cloud", "cloud-sun", "sun", "moon", "git-pull-request", "git-branch", "git-commit-horizontal",
	"terminal", "code", "bug", "rocket", "server", "globe", "mail", "message-square", "inbox",
	"list-todo", "square-check", "star", "heart", "flame", "eye", "lock", "shield", "sparkles",
	"bot", "clipboard", "sticky-note", "folder", "download", "upload", "refresh-cw", "chart-line",
	"gauge", "thermometer", "droplet", "house", "briefcase", "dollar-sign",
}

// KnownMenuBarIcon reports whether name is in the curated set.
func KnownMenuBarIcon(name MenuBarIcon) bool {
	for _, icon := range MenuBarIcons {
		if icon == name {
			return true
		}
	}
	return false
}
