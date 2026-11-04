// Package folders names the directories the app reads and writes. The shared folder is the one 1.x also uses: it
// holds config.json and extensions/, and nothing private or machine-local goes there, because the owner keeps it in
// a public dotfiles repository. The local folder is the operating system's user-data folder for everything that
// stays on this machine. Every function is pure: the caller passes the home folder, the environment and the app name.
package folders

import "path/filepath"

// PortableConfigName is the file of portable settings that 1.x and 2.0 both read.
const PortableConfigName = "config.json"

// ExtensionsName is the folder of user and AI-built extensions, shared with 1.x.
const ExtensionsName = "extensions"

// Shared is ~/.config/nevermind on every operating system.
func Shared(home string) string {
	return filepath.Join(home, ".config", "nevermind")
}

// Local is the operating system's user-data folder for the app's machine-local state: ~/Library/Application Support on
// macOS, %APPDATA% on Windows, and $XDG_DATA_HOME or ~/.local/share on Linux. name names the folder on macOS and
// Windows, and slug names it on Linux. MyGo's PathUserData puts Linux data under ~/.config, so it is not used there.
func Local(goos, home string, getenv func(string) string, name, slug string) string {
	switch goos {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", name)
	case "windows":
		return filepath.Join(appDataOf(home, getenv), name)
	}
	return filepath.Join(dataHomeOf(home, getenv), slug)
}

func appDataOf(home string, getenv func(string) string) string {
	if dir := getenv("APPDATA"); dir != "" {
		return dir
	}
	return filepath.Join(home, "AppData", "Roaming")
}

func dataHomeOf(home string, getenv func(string) string) string {
	if dir := getenv("XDG_DATA_HOME"); filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(home, ".local", "share")
}
