// Package folders names the directories the app reads and writes. The shared folder is the one 1.x also uses: it
// holds config.json and extensions/, and nothing private or machine-local goes there, because the owner keeps it in
// a public dotfiles repository. The local folder is the operating system's user-data folder for everything that
// stays on this machine. Every function is pure: the caller passes the home folder, the environment and the app name.
package folders

import (
	"path"
	"path/filepath"
)

// PortableConfigName is the file of portable settings that 1.x and 2.0 both read.
const PortableConfigName = "config.json"

// ExtensionsName is the folder of user and AI-built extensions, shared with 1.x.
const ExtensionsName = "extensions"

// Shared is ~/.config/nevermind on every operating system.
func Shared(home string) string {
	return filepath.Join(home, ".config", "nevermind")
}

// spelling is the path package of one operating system. The folder functions take goos rather than the host's OS, so
// they spell paths the way goos does: POSIX for macOS and Linux, Windows for Windows.
type spelling struct {
	join  func(elem ...string) string
	isAbs func(string) bool
}

func spellingOf(goos string) spelling {
	if goos == "windows" {
		return spelling{join: filepath.Join, isAbs: filepath.IsAbs}
	}
	return spelling{join: path.Join, isAbs: path.IsAbs}
}

// Local is the operating system's user-data folder for the app's machine-local state: ~/Library/Application Support on
// macOS, %APPDATA% on Windows, and $XDG_DATA_HOME or ~/.local/share on Linux. name names the folder on macOS and
// Windows, and slug names it on Linux. MyGo's PathUserData puts Linux data under ~/.config, so it is not used there.
func Local(goos, home string, getenv func(string) string, name, slug string) string {
	sp := spellingOf(goos)
	switch goos {
	case "darwin":
		return sp.join(home, "Library", "Application Support", name)
	case "windows":
		return sp.join(appDataOf(sp, home, getenv), name)
	}
	return sp.join(dataHomeOf(sp, home, getenv), slug)
}

// ElectronUserData is the user-data folder 1.x (an Electron app named name) kept its state in: ~/Library/Application
// Support on macOS, %APPDATA% on Windows, and $XDG_CONFIG_HOME or ~/.config on Linux.
func ElectronUserData(goos, home string, getenv func(string) string, name string) string {
	sp := spellingOf(goos)
	switch goos {
	case "darwin":
		return sp.join(home, "Library", "Application Support", name)
	case "windows":
		return sp.join(appDataOf(sp, home, getenv), name)
	}
	return sp.join(configHomeOf(sp, home, getenv), name)
}

func configHomeOf(sp spelling, home string, getenv func(string) string) string {
	if dir := getenv("XDG_CONFIG_HOME"); sp.isAbs(dir) {
		return dir
	}
	return sp.join(home, ".config")
}

func appDataOf(sp spelling, home string, getenv func(string) string) string {
	if dir := getenv("APPDATA"); dir != "" {
		return dir
	}
	return sp.join(home, "AppData", "Roaming")
}

func dataHomeOf(sp spelling, home string, getenv func(string) string) string {
	if dir := getenv("XDG_DATA_HOME"); sp.isAbs(dir) {
		return dir
	}
	return sp.join(home, ".local", "share")
}
