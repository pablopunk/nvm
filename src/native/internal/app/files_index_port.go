package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// fileIndexPort is the file index as the extension services read it. The file service starts after the extensions,
// so each call looks the service up when it runs; until it exists the index answers nothing.
type fileIndexPort struct{ p *App }

var _ services.FileIndex = fileIndexPort{}

// Roots are the index's default roots: the settings' roots, or the home folders when the settings name none.
func (f fileIndexPort) Roots() []string {
	home, _ := os.UserHomeDir()
	roots := f.p.S.FileRoots
	if len(roots) == 0 {
		roots = files.DefaultRoots(home)
	}
	return files.NormalizeRoots(home, roots)
}

// Search returns the best results of a query. The index streams batches; the last one is the answer.
func (f fileIndexPort) Search(ctx context.Context, query files.Query) []files.Result {
	if f.p.files == nil {
		return nil
	}
	var last []files.Result
	for batch := range f.p.files.ix.Search(ctx, query) {
		last = batch.Results
	}
	return last
}

// Status describes the index. Until the file service exists the index is not ready.
func (f fileIndexPort) Status() files.Status {
	if f.p.files == nil {
		return files.Status{}
	}
	return f.p.files.ix.Status()
}

// Rebuild starts a rescan and returns the entries the index holds now.
func (f fileIndexPort) Rebuild() int {
	if f.p.files == nil {
		return 0
	}
	f.p.files.ix.Refresh()
	return f.p.files.ix.Status().Entries
}

// appDescriptor names an application by its bundle path, for the shell calls that take one.
func appDescriptor(path string) platform.App {
	return platform.App{Path: path, Name: filepath.Base(path)}
}
