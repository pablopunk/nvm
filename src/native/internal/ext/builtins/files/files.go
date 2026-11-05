// Package files is the built-in that searches the file index: the recent files at the root, the typed file search at
// the root, the Search Files view and Find Files by Meaning. Rows open, preview, Quick Look and reveal their file
// through the declarative actions, and drag out as files. Find Files by Meaning asks the decision model about the
// names of the most recent files, only when the user asks, and only for that query.
//
// Layer: ext. The package imports api, search and the standard library.
package files

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// ID is the built-in's identity.
const ID = "nevermind.files"

// Limits of the rows, as the native file rows had them: the root shows the best few matches and the recent files at the
// empty query, the Search Files view a longer list, and Find Files by Meaning the recent names the model judges.
const (
	rootLimit         = 6
	viewLimit         = 100
	meaningLocal      = 16
	meaningCandidates = 16
	thumbnailRows     = 48
	minQueryChars     = 2
)

// Extension is the files built-in. It keeps the state the Search Files view and Find Files by Meaning share between
// their search calls: the query and kind last searched, and the decision the user asked for.
type Extension struct {
	state *viewState
}

// New returns the files built-in.
func New() Extension {
	return Extension{state: &viewState{}}
}

// viewState is what the file views remember between calls. Calls are serialized by the host, but the lock keeps the
// state safe if a later refresh runs beside a search.
type viewState struct {
	mu         sync.Mutex
	query      string
	kind       string
	askedQuery string
	rows       []api.Item
}

// Manifest identifies the built-in. Its rows are listed under the Files section.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: ID, Title: "Files"},
		Capabilities: []api.Capability{api.CapabilityDesktopFiles, api.CapabilitySystem, api.CapabilityAI},
	}
}

// RootItems lists the six newest files at the empty root, as 1.x lists its recent files.
func (Extension) RootItems(ctx api.Context) ([]api.Item, error) {
	found := recentFiles(ctx, rootLimit, "")
	items := make([]api.Item, 0, len(found))
	for i, file := range found {
		items = append(items, fileItem(ctx, file, float64(rootLimit-i)))
	}
	return items, nil
}

// SearchItems finds the files whose names match a typed query at the root, scored as the recent files are. Queries
// shorter than two characters find nothing, as 1.x's file search waits for them.
func (Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if len(query) < minQueryChars {
		return nil, nil
	}
	found := searchFiles(ctx, rootLimit, query, "")
	items := make([]api.Item, 0, len(found))
	for i, file := range found {
		items = append(items, fileItem(ctx, file, float64(rootLimit-i)))
	}
	return items, nil
}
