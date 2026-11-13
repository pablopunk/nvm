package app

import (
	"context"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/store"
)

// clipImageDirName is the folder, under the app's local data folder, that holds the copied pictures.
const (
	clipImageDirName = "clipboard-images"
	clipDefaultLimit = 300
)

// clipEngine is the clipboard history of the app: the history and its picture files, the watcher that captures
// copies, the copy and paste actions, the link previews and the picture cache of the detail pane. The features
// package holds the rules; this type wires them to the platform ports and to the palette's UI thread.
type clipEngine struct {
	history  *clipboard.History
	images   *clipboard.ImageStore
	files    *clipImageFiles
	watcher  *clipboard.Watcher
	actions  *clipboard.Actions
	links    *clipboard.LinkService
	linksOn  bool
	caps     platform.Capabilities
	clock    platform.Clock
	log      *logx.Logger
	store    *store.Store
	redraw   func(fn func())
	stop     context.CancelFunc
	stopped  <-chan struct{}
	watching bool
}

// clipDeps are what a clipboard engine is built from. Ports come from the composition root (the real set at
// launch, the fakes in tests). Dir is the app's local data folder, where the picture files go. Store is the app's one
// store (nil keeps the history in memory). Redraw runs a function on the palette's UI thread and redraws.
type clipDeps struct {
	ports   platform.Ports
	dir     string
	limit   int
	store   *store.Store
	redraw  func(fn func())
	log     *logx.Logger
	noLinks bool
}

// newClipEngine builds the engine. The history is a document of the given store; without a store, or when the
// document cannot be opened, the history is kept in memory for the session and the failure is logged, so the
// palette still works and nothing is written where it cannot be read.
func newClipEngine(deps clipDeps) *clipEngine {
	ports := deps.ports
	persist := clipboard.Persistence(clipboard.NewMemoryPersistence())
	if deps.store != nil {
		persist = clipboard.NewStorePersistence(clipboard.NewSnapshotDocument(deps.store))
	}
	history, err := clipboard.Open(persist, ports.Clock, clipRetention(deps.limit))
	if err != nil && history == nil {
		deps.log.Error("clipboard.history.open.failed", logx.F{"error": err.Error()})
		history, _ = clipboard.Open(clipboard.NewMemoryPersistence(), ports.Clock, clipRetention(deps.limit))
	}
	images := clipboard.NewImageStore(clipboard.ImageStoreDeps{
		Files: ports.FS, Clock: ports.Clock, Dir: filepath.Join(deps.dir, clipImageDirName), Log: deps.log,
	})
	watcher := clipboard.NewWatcher(clipboard.WatcherDeps{
		Clipboard: ports.Clipboard, Files: ports.FS, Capabilities: ports.Capabilities, Clock: ports.Clock,
		History: history, Images: images, Log: deps.log,
	})
	links := clipboard.NewLinkService(clipboard.LinkDeps{
		Fetcher: clipboard.NewLinkFetcher(ports.PublicHTTP, ports.Resolver), Images: images, Store: history,
		Clock: ports.Clock, OnSettled: func(string) { deps.redraw(func() {}) },
	})
	engine := &clipEngine{
		history: history,
		images:  images,
		watcher: watcher,
		actions: clipboard.NewActions(ports.Clipboard, ports.Paster, images, history, watcher),
		links:   links,
		linksOn: !deps.noLinks,
		caps:    ports.Capabilities,
		clock:   ports.Clock,
		log:     deps.log,
		store:   deps.store,
		redraw:  deps.redraw,
	}
	engine.files = newClipImageFiles(engine, deps.dir)
	return engine
}

// clipRetention is the policy of the history for a limit of unpinned entries. Entries never expire by age, as
// in 1.x; pinned entries are never removed by retention.
func clipRetention(limit int) clipboard.Retention {
	if limit <= 0 {
		limit = clipDefaultLimit
	}
	return clipboard.Retention{MaxEntries: limit, MaxAge: -1}
}

// setLimit applies a new limit of unpinned entries at once.
func (e *clipEngine) setLimit(limit int) {
	if _, err := e.history.SetRetention(clipRetention(limit)); err != nil {
		e.log.Warn("clipboard.retention.failed", logx.F{"error": err.Error()})
	}
}

// setLinkPreviews turns link previews on or off. Off, the detail pane draws no card and fetches nothing.
func (e *clipEngine) setLinkPreviews(on bool) {
	e.linksOn = on
}

// setEnabled starts or stops the watcher. Stopping cancels its loop; starting runs a new loop after the old one
// has exited, so two loops never read the clipboard at once.
func (e *clipEngine) setEnabled(on bool) {
	if on == e.watching {
		return
	}
	e.watching = on
	if !on {
		e.stop()
		e.stop = nil
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	previous := e.stopped
	e.stop, e.stopped = cancel, done
	safego.Go("clipboard.watcher.start", func(context.Context) {
		defer close(done)
		if previous != nil {
			<-previous
		}
		e.watcher.Run(ctx)
	})
}

// flush writes the history and the log now, while the store is still open. The app calls it when it quits.
func (e *clipEngine) flush() {
	if e.store != nil {
		if err := e.store.Flush(); err != nil {
			e.log.Error("clipboard.flush.failed", logx.F{"error": err.Error()})
		}
	}
	_ = e.log.Flush()
}

// openClipboard builds the engine over the app's one store, which ready opened with the directory's lease. Without
// the lease the history stays in memory for the session and nothing is watched: a second instance must not write
// pictures into a folder another instance owns. The watcher starts unless the user turned clipboard history off.
func (p *App) openClipboard() *clipEngine {
	var st *store.Store
	if !p.userReadOnly {
		st = p.userStore
	}
	e := newClipEngine(clipDeps{
		ports:   p.ports,
		dir:     dataDir(),
		limit:   p.S.ClipLimit,
		store:   st,
		redraw:  p.redraw,
		log:     logx.Default(),
		noLinks: p.S.ClipNoLinks,
	})
	e.setEnabled(!p.S.ClipOff && !p.userReadOnly)
	return e
}
