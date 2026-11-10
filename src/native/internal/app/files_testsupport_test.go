package app

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// newTestFiles returns a file service that already serves paths and touches no real file, folder, clipboard or desktop.
// Its index is the fixture index of files.NewIndexOfPaths: it never scans or watches.
func newTestFiles(tb testing.TB, home string, paths []string) *fileService {
	tb.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	tb.Cleanup(cancel)
	fsys := fake.NewFS()
	clock := fake.NewClock()
	ix, err := files.NewIndexOfPaths(fsys, clock, home, paths)
	if err != nil {
		tb.Fatal(err)
	}
	return &fileService{
		ix:           ix,
		fsys:         fsys,
		shell:        fake.NewShell(),
		clock:        clock,
		clipboard:    fake.NewClipboard(),
		copyFiles:    func(...string) error { return nil },
		home:         home,
		labels:       platform.ShellLabelsFor("linux"),
		thumbDir:     "/cache/files-thumbs",
		icons:        newAsyncCache[*ui.Bitmap](ctx, fileWorkers, fileQueue, fileIconEntries, func() {}),
		thumbs:       newAsyncCache[*ui.Bitmap](ctx, fileWorkers, fileQueue, fileThumbEntries, func() {}),
		gridPictures: newAsyncCache[*ui.Bitmap](ctx, 0, 1, gridPictureEntries, nil),
		redraw:       func() {},
		ctx:          ctx,
		log:          logx.Default(),
	}
}

// searchFilePaths returns the paths of the best limit results of query, as the Files view would list them.
func searchFilePaths(svc *fileService, query string, limit int) []string {
	var out []string
	for batch := range svc.ix.Search(context.Background(), files.Query{Text: query, Limit: limit}) {
		out = out[:0]
		for _, r := range batch.Results {
			out = append(out, r.Path)
		}
	}
	return out
}

// recordingHost is a search.Host for tests: it records what an action asked of the palette.
type recordingHost struct {
	hidden  bool
	toasts  []string
	pushed  []any
	refresh int
}

// Hide records that the palette closed.
func (h *recordingHost) Hide() { h.hidden = true }

// Pop does nothing: no test here leaves a view.
func (h *recordingHost) Pop() {}

// Push records the view an action opened.
func (h *recordingHost) Push(view any) { h.pushed = append(h.pushed, view) }

// Toast records a message.
func (h *recordingHost) Toast(msg string) { h.toasts = append(h.toasts, msg) }

// Refresh counts refreshes.
func (h *recordingHost) Refresh() { h.refresh++ }

// Query returns the empty query.
func (h *recordingHost) Query() string { return "" }

// SetQuery does nothing.
func (h *recordingHost) SetQuery(string) {}

var _ search.Host = (*recordingHost)(nil)

// Root has nothing to leave in this test host.
func (h *recordingHost) Root() {}
