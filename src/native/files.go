package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// FileIndex is an in-memory list of file paths under $HOME, built in the background.
type FileIndex struct {
	paths []string
	done  bool
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
	f.paths, f.done = paths, true
}

func (f *FileIndex) search(q string, limit int) []string {
	lq := strings.ToLower(q)
	var starts, contains []string
	for _, p := range f.paths {
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

func (p *Palette) fileItem(path string) Item {
	home, _ := os.UserHomeDir()
	dir := strings.Replace(filepath.Dir(path), home, "~", 1)
	return Item{
		ID: path, Title: filepath.Base(path), Subtitle: dir, IconPath: path,
		Actions: []Action{
			{Title: "Open", Run: func(p *Palette) { p.Hide(); _ = mygo.Shell.OpenPath(path) }},
			{Title: "Show in Folder", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(p *Palette) { p.Hide(); mygo.Shell.ShowItemInFolder(path) }},
			{Title: "Copy Path", Label: "⌃C", Mods: mod, Key: ui.KeyC, Run: func(p *Palette) { mygo.Clipboard.WriteText(path); p.Hide() }},
			{Title: "Move to Trash", Label: "⌃⌫", Mods: mod, Key: ui.KeyBackspace, Run: func(p *Palette) {
				if err := mygo.Shell.TrashItem(path); err != nil {
					p.Toast("Trash failed: " + err.Error())
				} else {
					p.Toast("Moved to Trash")
				}
			}},
		},
	}
}

func (p *Palette) filesView() *View {
	return &View{
		Title: "Search Files", Placeholder: "Search Files", EmptyTitle: "No files found.", EmptySub: "Type a file name.",
		Status: func() string {
			if !p.files.done {
				return "Indexing…"
			}
			return itoa(len(p.files.paths)) + " files indexed"
		},
		Search: func(q string) []Item {
			if len(q) < 1 {
				return nil
			}
			var out []Item
			for _, path := range p.files.search(q, 40) {
				out = append(out, p.fileItem(path))
			}
			return out
		},
	}
}
