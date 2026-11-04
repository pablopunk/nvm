package app

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// configDir is the directory of the app's own files. Golden tests swap it for a fixed path.
var configDir = defaultConfigDir

func defaultConfigDir() string {
	d := appConfigFolder(runtime.GOOS, os.UserHomeDir, os.UserConfigDir)
	_ = os.MkdirAll(d, 0o755)
	return d
}

// appConfigFolder is the app's own folder for one OS: under the home folder, or on Windows under the per-user
// application data folder, where Electron keeps its userData. Without a config folder it falls back to the home.
func appConfigFolder(goos string, home, userConfig func() (string, error)) string {
	if goos == "windows" {
		if dir, err := userConfig(); err == nil {
			return filepath.Join(dir, "nvm-native")
		}
	}
	dir, _ := home()
	return filepath.Join(dir, ".config", "nvm-native")
}

func loadJSON(name string, v any) {
	data, err := os.ReadFile(filepath.Join(configDir(), name))
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, v); err != nil {
		log.Printf("load %s: %v", name, err)
	}
}

func saveJSON(name string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	path := filepath.Join(configDir(), name)
	tmp := path + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, path)
	}
}

// Settings are persisted in settings.json. What the user chose in the root is not here: it is the usage
// memory (usage.json) and the query history (queries.json), see learning.go.
type Settings struct {
	Hotkey      string
	Engine      string
	ClipOff     bool
	ClipLimit   int
	ClipNoLinks bool // link previews of copied web addresses are off
	Quicklinks  []Quicklink
	Disabled    map[string]bool // extension ids
	LoginAtOpen bool
	// DiagnosticsOff is the "Send Technical Diagnostics" switch, inverted so that a missing key means on.
	DiagnosticsOff bool
	// EmojiSkinTone is the skin tone the emoji picker applies, as emoji.SkinTone (0 is the default tone).
	EmojiSkinTone int
	// FileRoots are the folders the Files index covers; empty means the standard personal folders.
	FileRoots []string
	// FileIgnore adds ignore patterns to the Files index, on top of the defaults.
	FileIgnore []string
	// GuideDone is set once the user dismisses the Getting Started guide; the root then shows Help.
	GuideDone bool
}

type Quicklink struct {
	Name, Keyword, URL string // URL has {query}
}

func loadSettings() *Settings {
	s := &Settings{Engine: "google", ClipLimit: clipDefaultLimit, Disabled: map[string]bool{}}
	loadJSON("settings.json", s)
	if s.Disabled == nil {
		s.Disabled = map[string]bool{}
	}
	if s.ClipLimit <= 0 {
		s.ClipLimit = clipDefaultLimit
	}
	return s
}

func (s *Settings) save() { saveJSON("settings.json", s) }
