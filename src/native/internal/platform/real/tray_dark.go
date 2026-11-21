package real

import (
	"os"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
)

// alwaysDarkPanels are the desktops whose panels stay dark whatever the app's theme is.
var alwaysDarkPanels = []string{"gnome", "unity", "ubuntu", "pantheon", "budgie"}

// barIsDark reports whether the bar an icon sits on is dark. The macOS menu bar and the Windows taskbar say so
// themselves. Linux panels do not, so the app's theme stands in, and the GNOME family is always dark.
func barIsDark(tray *mygo.Tray) bool {
	if runtime.GOOS != "linux" {
		return tray.Dark()
	}
	return panelIsAlwaysDark(os.Getenv("XDG_CURRENT_DESKTOP")) || mygo.Theme.IsDark()
}

// panelIsAlwaysDark matches a desktop name, such as "ubuntu:GNOME" from XDG_CURRENT_DESKTOP.
func panelIsAlwaysDark(desktop string) bool {
	lower := strings.ToLower(desktop)
	for _, name := range alwaysDarkPanels {
		if strings.Contains(lower, name) {
			return true
		}
	}
	return false
}
