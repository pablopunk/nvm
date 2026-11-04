package app

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pablopunk/nvm/native/internal/platform/folders"
)

// The app's folder names: the macOS and Windows data folder is named after the app, and the Linux one after its slug.
const (
	appFolderName = "Nevermind Next"
	appFolderSlug = "nevermind-next"
)

// dataDir is the machine-local folder: the operating system's user-data folder, holding everything that never leaves
// this machine. Golden tests swap it for a fixed path.
var dataDir = defaultDataDir

func defaultDataDir() string {
	home, _ := os.UserHomeDir()
	d := folders.Local(runtime.GOOS, home, os.Getenv, appFolderName, appFolderSlug)
	_ = os.MkdirAll(d, 0o755)
	return d
}

func loadJSON(name string, v any) {
	data, err := os.ReadFile(filepath.Join(dataDir(), name))
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
	path := filepath.Join(dataDir(), name)
	tmp := path + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, path)
	}
}

// Settings are persisted in settings.json, in the local folder. The settings 1.x shares (the palette shortcut, the
// diagnostics switch, the update channel) are read from config.json, see shared_config.go. What the user chose in the
// root is not here: it is the usage memory (usage.json) and the query history (queries.json), see learning.go.
type Settings struct {
	Engine      string
	ClipOff     bool
	ClipLimit   int
	ClipNoLinks bool // link previews of copied web addresses are off
	// ClipRootOff and ClipRootSearchOff hide the copies from the root list and from the root's search. They are
	// inverted so that a missing key means shown, as 1.x's defaults do.
	ClipRootOff       bool
	ClipRootSearchOff bool
	Quicklinks        []Quicklink
	Disabled          map[string]bool // extension ids
	LoginAtOpen       bool
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
