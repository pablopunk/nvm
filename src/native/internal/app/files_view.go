package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// fileKindFilter is one state of the Files view's kind filter. Tab steps through them in order.
type fileKindFilter struct {
	label string
	kinds []files.Kind
}

// fileKindFilters are the kinds the Files view offers. Folders are not listed: the index holds files only.
var fileKindFilters = []fileKindFilter{
	{label: "All files"},
	{label: "Images", kinds: []files.Kind{files.KindImage}},
	{label: "Videos", kinds: []files.Kind{files.KindVideo}},
	{label: "Media", kinds: []files.Kind{files.KindMedia}},
}

// filesView is the Search Files view. Its filter starts at "All files" each time the view opens, because filters reset
// during navigation. An empty query lists the recent files; a typed query searches the names.
func (p *App) filesView() *palette.View {
	return p.files.browseView()
}

// browseView builds the Search Files view over the index.
func (s *fileService) browseView() *palette.View {
	filter := 0
	v := &palette.View{
		Title:       "Search Files",
		Placeholder: "Search Files",
		EmptyTitle:  s.emptyTitle(),
		EmptySub:    "Type a file name. Press Tab to change the kind.",
		Hint:        "Tab kind",
	}
	v.Providers = func() []search.Provider {
		return []search.Provider{fileSearch{svc: s, limit: fileViewLimit, kinds: fileKindFilters[filter].kinds, recentEmpty: true, id: "files-view"}}
	}
	v.Status = func() string {
		return fileKindFilters[filter].label + " · " + s.statusLabel()
	}
	v.OnKey = func(c *ui.Context, h search.Host) bool {
		if !c.Shortcut(0, ui.KeyTab) {
			return false
		}
		filter = (filter + 1) % len(fileKindFilters)
		h.Refresh()
		return true
	}
	return v
}
