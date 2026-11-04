package app

import (
	"context"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Limits of the file rows. The root shows the best few matches; the Files view shows a longer list.
const (
	fileRootLimit = 4
	fileViewLimit = 100
	// fileRootMinChars is how many characters the root needs before it searches files.
	fileRootMinChars = 2
)

// fileSearch is a streaming provider over the file index. The root uses one for typed queries; the Files view uses
// one for its kind filter, with the recent files answering the empty query. Results stream in batches as the index
// finds them, so the first rows show before the search completes.
type fileSearch struct {
	svc         *fileService
	limit       int
	minChars    int
	kinds       []files.Kind
	recentEmpty bool
	id          string
}

// rootFileProvider is the root's files source.
func rootFileProvider(p *App, _ *rootInput) search.Provider {
	return fileSearch{svc: p.files, limit: fileRootLimit, minChars: fileRootMinChars, id: "files"}
}

// ID names the provider in statistics.
func (f fileSearch) ID() string { return f.id }

// Root answers the empty query: the recent files for the Files view, nothing for the root.
func (f fileSearch) Root(ctx context.Context) []search.Item {
	if !f.recentEmpty {
		return nil
	}
	var out []search.Item
	f.run(ctx, "", func(b search.Batch) { out = b.Items })
	return out
}

// Search emits the rows for a typed query as the index streams them.
func (f fileSearch) Search(ctx context.Context, q string, emit func(search.Batch)) {
	if len(q) < f.minChars {
		emit(search.Batch{Final: true})
		return
	}
	f.run(ctx, q, emit)
}

// run asks the index for the best results of text and emits their rows, one batch per index batch.
func (f fileSearch) run(ctx context.Context, text string, emit func(search.Batch)) {
	q := files.Query{Text: text, Kinds: f.kinds, Limit: f.limit}
	if text == "" {
		q.SortBy = files.SortRecent
	}
	for batch := range f.svc.ix.Search(ctx, q) {
		emit(search.Batch{Items: f.items(batch.Results), Final: batch.Final})
	}
}

// items turns index results into rows. Scores keep the index order: the first result ranks first among the files.
func (f fileSearch) items(results []files.Result) []search.Item {
	out := make([]search.Item, 0, len(results))
	for i, r := range results {
		it := f.svc.fileItem(r)
		it.Kind = search.KindFiles
		it.Score = f.scoreFor(i)
		out = append(out, it)
	}
	return out
}

// scoreFor ranks the i-th result. The root keeps its band below the calculator and quicklinks; the view ranks by order.
func (f fileSearch) scoreFor(i int) int {
	if f.recentEmpty {
		return f.limit - i
	}
	return scoreFileFirst - i
}

// fileKindIcon is the icon of a file row until the system's icon of the file arrives, or when the system has none.
// Without it the palette would spell the name's first letters.
func fileKindIcon(kind files.Kind) string {
	switch kind {
	case files.KindImage:
		return "image"
	case files.KindVideo:
		return "play"
	default:
		return "file-text"
	}
}

// fileItem is the row of one file: its name, its folder, its picture, and the actions a user takes on it.
func (s *fileService) fileItem(r files.Result) search.Item {
	return search.Item{
		ID:           r.Path,
		Title:        r.Name,
		Subtitle:     filepath.Dir(r.DisplayPath),
		IconPath:     r.Path,
		IconFallback: fileKindIcon(r.Kind),
		DragPaths:    []string{r.Path},
		Detail:       s.detail(r),
		Actions:      s.actionsFor(r),
	}
}
