package app

import (
	"os"
	"strings"
)

// displayPath shortens a path under the home folder to "~", for the labels the palette shows.
func (p *App) displayPath(path string) string {
	home, _ := os.UserHomeDir()
	if home != "" && (path == home || strings.HasPrefix(path, home+"/")) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
