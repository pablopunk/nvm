// Package devicemodel keeps the on-device speech model on disk: whether it is there, and its download, which runs
// once, in the background, resumes after an interruption and checks every file. The files are pinned by revision
// and checksum in the embedded manifest. Layer: service tier (with dictation).
package devicemodel

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

//go:embed parakeet-v3.json
var manifestJSON []byte

// File is one file of the model, as the manifest pins it.
type File struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Manifest names the model, where its files come from, and what they must be.
type Manifest struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Credit  string `json:"credit"`
	BaseURL string `json:"baseURL"`
	Files   []File `json:"files"`
	// Replaces names the folders of earlier versions of the model, removed once this one is on disk.
	Replaces []string `json:"replaces,omitempty"`
}

// Size is the bytes the model takes on disk.
func (m Manifest) Size() int64 {
	var total int64
	for _, file := range m.Files {
		total += file.Size
	}
	return total
}

// Parakeet is the model dictation runs on the device.
func Parakeet() Manifest {
	var manifest Manifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		panic("devicemodel: the embedded manifest does not parse: " + err.Error())
	}
	return manifest
}

// Status is where the model stands on this device.
type Status string

// The model's statuses.
const (
	StatusMissing     Status = "missing"
	StatusDownloading Status = "downloading"
	StatusReady       Status = "ready"
	StatusFailed      Status = "failed"
)

// State is what the settings show about the model. Progress runs from 0 to 1 while it downloads; Error says why the
// last download failed.
type State struct {
	Status   Status
	Progress float64
	Size     int64
	Error    string
}

// ErrNotReady is a transcription asked of a model that is not on disk yet.
var ErrNotReady = errors.New("the on-device speech model is not downloaded yet")

// Manager keeps one model under a folder. It is safe for concurrent use.
type Manager struct {
	files     platform.FS
	downloads platform.FileDownloader
	manifest  Manifest
	dir       string

	mu       sync.Mutex
	running  bool
	progress float64
	failure  string
}

// New returns the manager of manifest's model under root.
func New(files platform.FS, downloads platform.FileDownloader, root string, manifest Manifest) *Manager {
	return &Manager{files: files, downloads: downloads, manifest: manifest, dir: filepath.Join(root, manifest.Name)}
}

// Dir is the folder that holds the model's files.
func (m *Manager) Dir() string {
	return m.dir
}

// Manifest is the model this manager keeps.
func (m *Manager) Manifest() Manifest {
	return m.manifest
}

// State reads the model's status. A model whose files are all there at their pinned sizes is ready.
func (m *Manager) State() State {
	m.mu.Lock()
	running, progress, failure := m.running, m.progress, m.failure
	m.mu.Unlock()
	state := State{Size: m.manifest.Size()}
	switch {
	case running:
		state.Status, state.Progress = StatusDownloading, progress
	case m.onDisk():
		state.Status, state.Progress = StatusReady, 1
	case failure != "":
		state.Status, state.Error = StatusFailed, failure
	default:
		state.Status = StatusMissing
	}
	return state
}

// Ready reports whether the model can be loaded now.
func (m *Manager) Ready() bool {
	return m.State().Status == StatusReady
}

func (m *Manager) onDisk() bool {
	for _, file := range m.manifest.Files {
		info, err := m.files.Stat(filepath.Join(m.dir, file.Name))
		if err != nil || info.Size() != file.Size {
			return false
		}
	}
	return true
}

// Install starts the download in the background, unless the model is ready or already downloading. background runs
// the download; it outlives the caller, as the download takes minutes.
func (m *Manager) Install(background func(name string, fn func())) {
	m.mu.Lock()
	if m.running || m.onDisk() {
		m.mu.Unlock()
		return
	}
	m.running, m.progress, m.failure = true, 0, ""
	m.mu.Unlock()
	background("dictation.download-model", func() {
		err := m.download(context.Background())
		if err == nil {
			m.removeReplaced()
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		m.running = false
		if err != nil {
			m.failure = err.Error()
		}
	})
}

// download fetches each missing file in turn, reporting the share of the whole model on disk.
func (m *Manager) download(ctx context.Context) error {
	total := float64(m.manifest.Size())
	var finished int64
	for _, file := range m.manifest.Files {
		path := filepath.Join(m.dir, file.Name)
		if info, err := m.files.Stat(path); err == nil && info.Size() == file.Size {
			finished += file.Size
			continue
		}
		base := finished
		err := m.downloads.Download(ctx, m.manifest.BaseURL+"/"+file.Name, path, file.SHA256, func(done int64) {
			m.mu.Lock()
			m.progress = float64(base+done) / total
			m.mu.Unlock()
		})
		if err != nil {
			return err
		}
		finished += file.Size
	}
	return nil
}

// removeReplaced deletes the folders of earlier versions of the model, which nothing reads any more. A folder that
// cannot be removed is left; it only takes disk space.
func (m *Manager) removeReplaced() {
	root := filepath.Dir(m.dir)
	for _, name := range m.manifest.Replaces {
		removeTree(m.files, filepath.Join(root, name))
	}
}

// removeTree deletes a folder and everything in it through the file port, which removes one entry at a time.
func removeTree(files platform.FS, dir string) {
	entries, err := files.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			removeTree(files, path)
			continue
		}
		_ = files.Remove(path)
	}
	_ = files.Remove(dir)
}
