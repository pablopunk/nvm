package manager

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// warmEntryPoints are the calls the palette or the menu bar makes without the user asking for that extension. An
// extension that implements one keeps a running child, so those calls never wait for a start.
var warmEntryPoints = []string{
	protocol.EntryPoints[protocol.MethodRoot],
	protocol.EntryPoints[protocol.MethodSearch],
	protocol.EntryPoints[protocol.MethodMenuBars],
	protocol.EntryPoints[protocol.MethodMenuBarTriggers],
}

const commandSnapshotsFile = ".command-snapshots.json"

type commandSnapshot struct {
	Hash     string        `json:"hash"`
	Commands []api.Command `json:"commands"`
}

// commandSnapshots keeps each extension's commands on disk, keyed by the hash of its source, so an extension that only
// offers commands needs no child until one runs. A store with no path keeps them in memory.
type commandSnapshots struct {
	files platform.FS
	path  string

	mu   sync.Mutex
	byID map[string]commandSnapshot
}

func openCommandSnapshots(files platform.FS, dataRoot string) *commandSnapshots {
	store := &commandSnapshots{files: files, byID: map[string]commandSnapshot{}}
	if dataRoot == "" {
		return store
	}
	store.path = filepath.Join(dataRoot, commandSnapshotsFile)
	if data, err := files.ReadFile(store.path); err == nil {
		if json.Unmarshal(data, &store.byID) != nil || store.byID == nil {
			store.byID = map[string]commandSnapshot{}
		}
	}
	return store
}

func (s *commandSnapshots) get(id, hash string) ([]api.Command, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, ok := s.byID[id]
	if !ok || snapshot.Hash != hash {
		return nil, false
	}
	return snapshot.Commands, true
}

// put saves the commands when they differ from what is saved. A failed write only costs a child start next launch.
func (s *commandSnapshots) put(id, hash string, commands []api.Command) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := commandSnapshot{Hash: hash, Commands: commands}
	before, _ := json.Marshal(s.byID[id])
	after, _ := json.Marshal(next)
	if string(before) == string(after) {
		return
	}
	s.byID[id] = next
	if s.path == "" {
		return
	}
	data, err := json.Marshal(s.byID)
	if err != nil {
		return
	}
	temp := s.path + ".tmp"
	if s.files.MkdirAll(filepath.Dir(s.path), 0o700) != nil {
		return
	}
	if s.files.WriteFile(temp, data, 0o600) == nil {
		_ = s.files.Rename(temp, s.path)
	}
}

// startEntry starts an enabled extension: dormant when its commands are saved and nothing calls it unasked, otherwise
// with a child now.
func (m *Manager) startEntry(e *entry) {
	runtime, canSleep := e.runtime.(*supervisor.Runtime)
	if canSleep {
		commands, known := m.snapshots.get(e.bundle.Manifest.ID, e.bundle.Hash)
		if methods, dormant := dormantMethods(m.ports.Files, e.bundle.Dir); known && dormant {
			runtime.StartDormant(methods)
			m.commands.seed(e.bundle.Manifest.ID, CommandResult{ExtensionID: e.bundle.Manifest.ID, Title: e.bundle.Manifest.Title, Commands: commands})
			return
		}
	}
	e.runtime.Start()
}

// dormantMethods lists the methods the source in dir implements, and whether it can wait for its first call.
func dormantMethods(files platform.FS, dir string) ([]string, bool) {
	declared := bundle.DeclaredFunctions(files, dir)
	if declared == nil {
		return nil, false
	}
	for _, function := range warmEntryPoints {
		if declared[function] {
			return nil, false
		}
	}
	var methods []string
	for method, function := range protocol.EntryPoints {
		if declared[function] {
			methods = append(methods, method)
		}
	}
	sort.Strings(methods)
	return methods, true
}

func isDormant(target runtimeTarget) bool {
	return target.runtime.Status().State == supervisor.StateDormant
}
