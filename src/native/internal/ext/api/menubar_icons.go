package api

import "sync"

// MenuBarIcon is a status glyph, named by any Lucide icon in kebab-case such as bell or git-branch. Unknown names fall
// back to MenuIconDefault and produce a diagnostic.
type MenuBarIcon string

// MenuIconDefault is the fallback glyph.
const MenuIconDefault MenuBarIcon = "zap"

// MenuBarIcons is the glyph set 1.x offered, in the same order as the TypeScript declaration.
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

// KnownMenuBarIcon reports whether name is a Lucide icon the host can draw.
func KnownMenuBarIcon(name MenuBarIcon) bool {
	knownMenuBarIconsOnce.Do(indexMenuBarIcons)
	return knownMenuBarIcons[name]
}

var (
	knownMenuBarIconsOnce sync.Once
	knownMenuBarIcons     map[MenuBarIcon]bool
)

func indexMenuBarIcons() {
	knownMenuBarIcons = make(map[MenuBarIcon]bool, len(MenuBarIcons)+len(lucideMenuBarIcons))
	for _, icon := range MenuBarIcons {
		knownMenuBarIcons[icon] = true
	}
	for _, name := range lucideMenuBarIcons {
		knownMenuBarIcons[MenuBarIcon(name)] = true
	}
}
