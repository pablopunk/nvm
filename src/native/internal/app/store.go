package app

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/quicklinks"
	"github.com/pablopunk/nvm/native/internal/platform/folders"
)

// The app's folder names: the macOS and Windows data folder is named after the app, and the Linux one after its slug.
const (
	appFolderName = "Nevermind"
	appFolderSlug = "nevermind"
)

// The folder names the app had while it was called Nevermind Next. The first launch under the current name moves that
// folder over, so the owner's data survives the rename.
const (
	renamedFromFolderName = "Nevermind Next"
	renamedFromFolderSlug = "nevermind-next"
)

var adoptRenamedDataDirOnce sync.Once

// dataDir is the machine-local folder: the operating system's user-data folder, holding everything that never leaves
// this machine. Golden tests swap it for a fixed path.
var dataDir = defaultDataDir

func defaultDataDir() string {
	home, _ := os.UserHomeDir()
	d := folders.Local(runtime.GOOS, home, os.Getenv, appFolderName, appFolderSlug)
	adoptRenamedDataDirOnce.Do(adoptRenamedDataDir)
	_ = os.MkdirAll(d, 0o755)
	return d
}

// adoptRenamedDataDir moves the Nevermind Next data folder to the current one when only the former exists.
func adoptRenamedDataDir() {
	home, _ := os.UserHomeDir()
	previous := folders.Local(runtime.GOOS, home, os.Getenv, renamedFromFolderName, renamedFromFolderSlug)
	current := folders.Local(runtime.GOOS, home, os.Getenv, appFolderName, appFolderSlug)
	moved, err := folders.AdoptRenamed(previous, current)
	switch {
	case err != nil:
		log.Printf("move data folder %s to %s: %v", previous, current, err)
	case moved:
		log.Printf("moved data folder %s to %s", previous, current)
	}
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
	// Engine is the search engine 2.0 kept here. The web built-in reads it as the default of its engine setting.
	Engine      string
	ClipOff     bool
	ClipLimit   int
	ClipNoLinks bool // link previews of copied web addresses are off
	// ClipRootOff and ClipRootSearchOff hide the copies from the root list and from the root's search. They are
	// inverted so that a missing key means shown, as 1.x's defaults do.
	ClipRootOff       bool
	ClipRootSearchOff bool
	// Quicklinks are the links 2.0 kept here. The quicklinks built-in reads them until the user creates a link, which
	// stores them in the built-in's own storage.
	// EmojiSkinTone is the skin tone the emoji picker applied before the built-in kept its own. The built-in reads it until
	// its storage holds a tone.
	EmojiSkinTone int
	Quicklinks    []quicklinks.Link
	Disabled      map[string]bool // extension ids
	LoginAtOpen   bool
	// DiagnosticsOff is the "Send Technical Diagnostics" switch, inverted so that a missing key means on.
	DiagnosticsOff bool
	// FileRoots are the folders the Files index covers; empty means the standard personal folders.
	FileRoots []string
	// FileIgnore adds ignore patterns to the Files index, on top of the defaults.
	FileIgnore []string
	// GuideDone is set once the user dismissed the Getting Started guide. The guide built-in reads it until its own
	// storage holds the flag.
	GuideDone bool
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
