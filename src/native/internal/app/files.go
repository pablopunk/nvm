package app

import (
	"github.com/pablopunk/nvm/native/internal/search"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// FileIndex is an in-memory list of file paths under $HOME, built in the background. Its paths are set once,
// under mu, and never edited, so a provider may search the slice it read with snapshot.
type FileIndex struct {
	mu    sync.Mutex
	paths []string
	done  bool
}

// snapshot returns the indexed paths and whether the index is built yet.
func (f *FileIndex) snapshot() ([]string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.paths, f.done
}

var skipDirs = map[string]bool{"node_modules": true, "target": true, "vendor": true, "__pycache__": true, "go": true, "venv": true, "dist": true, "build": true}

func (f *FileIndex) build() {
	home, _ := os.UserHomeDir()
	var paths []string
	_ = filepath.WalkDir(home, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if path != home && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() && skipDirs[name] {
			return filepath.SkipDir
		}
		if rel, _ := filepath.Rel(home, path); strings.Count(rel, string(filepath.Separator)) > 6 {
			return filepath.SkipDir
		}
		if len(paths) >= 60000 {
			return filepath.SkipAll
		}
		if path != home {
			paths = append(paths, path)
		}
		return nil
	})
	f.mu.Lock()
	f.paths, f.done = paths, true
	f.mu.Unlock()
}

func (f *FileIndex) search(q string, limit int) []string {
	paths, _ := f.snapshot()
	return searchPaths(paths, q, limit)
}

// searchPaths returns up to limit paths whose base name starts with q, then those that contain it.
func searchPaths(paths []string, q string, limit int) []string {
	lq := strings.ToLower(q)
	var starts, contains []string
	for _, p := range paths {
		base := strings.ToLower(filepath.Base(p))
		switch {
		case strings.HasPrefix(base, lq):
			starts = append(starts, p)
		case strings.Contains(base, lq):
			contains = append(contains, p)
		}
		if len(starts) >= limit {
			break
		}
	}
	return append(starts, contains...)[:min(limit, len(starts)+len(contains))]
}

func (p *App) fileItem(path string) search.Item {
	home, _ := os.UserHomeDir()
	dir := strings.Replace(filepath.Dir(path), home, "~", 1)
	return search.Item{
		ID: path, Title: filepath.Base(path), Subtitle: dir, IconPath: path,
		Actions: []search.Action{
			{Title: "Open", Run: func(h search.Host) { h.Hide(); _ = mygo.Shell.OpenPath(path) }},
			{Title: "Show in Folder", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(h search.Host) { h.Hide(); mygo.Shell.ShowItemInFolder(path) }},
			{Title: "Copy Path", Label: "⌃C", Mods: mod, Key: ui.KeyC, Run: func(h search.Host) { mygo.Clipboard.WriteText(path); h.Hide() }},
			{Title: "Move to Trash", Label: "⌃⌫", Mods: mod, Key: ui.KeyBackspace, Dismiss: search.DismissNever, Run: func(h search.Host) {
				if err := mygo.Shell.TrashItem(path); err != nil {
					h.Toast("Trash failed: " + err.Error())
				} else {
					h.Toast("Moved to Trash")
				}
			}},
		},
	}
}

func (p *App) filesView() *palette.View {
	return &palette.View{
		Title: "Search Files", Placeholder: "Search Files", EmptyTitle: "No files found.", EmptySub: "Type a file name.",
		Status: func() string {
			if _, done := p.files.snapshot(); !done {
				return "Indexing…"
			}
			paths, _ := p.files.snapshot()
			return itoa(len(paths)) + " files indexed"
		},
		Search: func(q string) []search.Item {
			if len(q) < 1 {
				return nil
			}
			var out []search.Item
			for _, path := range p.files.search(q, 40) {
				out = append(out, p.fileItem(path))
			}
			return out
		},
	}
}
