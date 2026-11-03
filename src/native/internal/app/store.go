package app

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// configDir is the directory of the app's own files. Golden tests swap it for a fixed path.
var configDir = defaultConfigDir

func defaultConfigDir() string {
	home, _ := os.UserHomeDir()
	d := filepath.Join(home, ".config", "nvm-native")
	_ = os.MkdirAll(d, 0o755)
	return d
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
	Quicklinks  []Quicklink
	Disabled    map[string]bool // extension ids
	LoginAtOpen bool
	// EmojiSkinTone is the skin tone the emoji picker applies, as emoji.SkinTone (0 is the default tone).
	EmojiSkinTone int
}

type Quicklink struct {
	Name, Keyword, URL string // URL has {query}
}

func loadSettings() *Settings {
	s := &Settings{Hotkey: "Alt+Shift+Space", Engine: "google", ClipLimit: 200, Disabled: map[string]bool{}}
	loadJSON("settings.json", s)
	if s.Disabled == nil {
		s.Disabled = map[string]bool{}
	}
	if s.ClipLimit <= 0 {
		s.ClipLimit = 200
	}
	return s
}

func (s *Settings) save() { saveJSON("settings.json", s) }
