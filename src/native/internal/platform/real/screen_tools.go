package real

// screenTool is a Linux program that saves the whole screen to a PNG without asking anything.
type screenTool struct {
	binary  string
	args    func(dst string) []string
	x11     bool
	wayland bool
}

// linuxScreenTools are tried in order. Each desktop ships at least one of them, and the first that fits the session wins.
var linuxScreenTools = []screenTool{
	{binary: "grim", wayland: true, args: func(dst string) []string { return []string{dst} }},
	{binary: "spectacle", x11: true, wayland: true, args: func(dst string) []string { return []string{"-b", "-n", "-f", "-o", dst} }},
	{binary: "gnome-screenshot", x11: true, wayland: true, args: func(dst string) []string { return []string{"-f", dst} }},
	{binary: "scrot", x11: true, args: func(dst string) []string { return []string{"-o", dst} }},
	{binary: "import", x11: true, args: func(dst string) []string { return []string{"-window", "root", dst} }},
}

// linuxScreenTool picks the capture program for this session. A session with no display has none.
func linuxScreenTool(getenv func(string) string, hasTool func(string) bool) (screenTool, bool) {
	wayland := isWaylandSession("linux", getenv)
	if !wayland && getenv("DISPLAY") == "" {
		return screenTool{}, false
	}
	for _, tool := range linuxScreenTools {
		if ((wayland && tool.wayland) || (!wayland && tool.x11)) && hasTool(tool.binary) {
			return tool, true
		}
	}
	return screenTool{}, false
}

// screenCaptureCapable reports whether the screen can be read: always on macOS, and with a capture program on Linux.
func screenCaptureCapable(goos string, getenv func(string) string, hasTool func(string) bool) bool {
	switch goos {
	case "darwin":
		return true
	case "linux":
		_, ok := linuxScreenTool(getenv, hasTool)
		return ok
	}
	return false
}
