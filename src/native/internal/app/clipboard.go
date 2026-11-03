package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/store"
)

// Names of the clipboard engine's background work and files.
const (
	clipNoticesName  = "clipboard.store.notices"
	clipImageDirName = "clipboard-images"
	clipLogFileName  = "nvm.log"
	clipDefaultLimit = 300
)

// clipEngine is the clipboard history of the app: the history and its picture files, the watcher that captures
// copies, the copy and paste actions, the link previews and the picture cache of the detail pane. The features
// package holds the rules; this type wires them to the platform ports and to the palette's UI thread.
type clipEngine struct {
	history  *clipboard.History
	images   *clipboard.ImageStore
	watcher  *clipboard.Watcher
	actions  *clipboard.Actions
	links    *clipboard.LinkService
	linksOn  bool
	pics     *clipPictures
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
// launch, the fakes in tests). Dir is the app's config folder. Redraw runs a function on the palette's UI thread
// and redraws; Toast shows a one-line message the same way. StoreClock times the store's debounced writes; nil
// means the ports' clock. Tests give the store its own clock so the watcher's timers are the only ones they move.
type clipDeps struct {
	ports      platform.Ports
	dir        string
	limit      int
	redraw     func(fn func())
	toast      func(msg string)
	log        *logx.Logger
	storeClock platform.Clock
	noLinks    bool
}

// newClipEngine builds the engine. A history that cannot be opened over the store is kept in memory for the
// session and the failure is logged, so the palette still works and nothing is written where it cannot be read.
func newClipEngine(deps clipDeps) *clipEngine {
	ports := deps.ports
	persist, st := openClipPersistence(deps)
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
	e := &clipEngine{
		history: history,
		images:  images,
		watcher: watcher,
		actions: clipboard.NewActions(ports.Clipboard, ports.Paster, images, history, watcher),
		links:   links,
		linksOn: !deps.noLinks,
		pics:    newClipPictures(images, deps.redraw, decodePicture),
		caps:    ports.Capabilities,
		clock:   ports.Clock,
		log:     deps.log,
		store:   st,
		redraw:  deps.redraw,
	}
	if st != nil {
		safego.Go(clipNoticesName, func(context.Context) { reportStoreNotices(st, deps.toast) })
	}
	return e
}

// openClipPersistence opens the app's store and the history document in it. Without a store (its folder cannot
// be created) the history is memory-only, and the returned store is nil.
func openClipPersistence(deps clipDeps) (clipboard.Persistence, *store.Store) {
	clock := deps.storeClock
	if clock == nil {
		clock = deps.ports.Clock
	}
	st, err := store.Open(deps.ports.FS, clock, deps.dir)
	if err != nil {
		deps.log.Error("clipboard.store.open.failed", logx.F{"error": err.Error()})
		return clipboard.NewMemoryPersistence(), nil
	}
	return clipboard.NewStorePersistence(clipboard.NewSnapshotDocument(st)), st
}

// reportStoreNotices shows the store's notices about the clipboard file. A quarantined file is named, never read
// out: the history starts empty and the old file stays where the store moved it.
func reportStoreNotices(st *store.Store, toast func(msg string)) {
	for notice := range st.Notices() {
		if notice.Kind == store.Quarantined {
			toast(fmt.Sprintf("Clipboard history could not be read and was moved aside: %s", notice.Path))
		}
	}
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

// flush writes the history and the log now. The app calls it when it quits.
func (e *clipEngine) flush() {
	if e.store != nil {
		if err := e.store.Flush(); err != nil {
			e.log.Error("clipboard.flush.failed", logx.F{"error": err.Error()})
		}
	}
	_ = e.log.Flush()
}

// copy puts an entry on the clipboard and records the use.
func (e *clipEngine) copy(entry clipboard.Entry) error {
	return e.actions.Copy(entry)
}

// paste puts an entry on the clipboard and sends the paste gesture to the app that had focus.
func (e *clipEngine) paste(entry clipboard.Entry) error {
	return e.actions.Paste(entry, "")
}

// textEntry describes plain text that did not come from the history, such as a calculation result, so it can be
// pasted through the same path. Its copy is suppressed from the history, as any copy the app makes is.
func textEntry(text string) clipboard.Entry {
	return clipboard.Entry{ID: clipboard.TextID(text), Kind: clipboard.KindText, Text: text}
}

// openClipboard builds the engine over the real ports at launch, with the paste gesture waiting for the
// palette to hide. It starts the watcher unless the user turned clipboard history off.
func (p *App) openClipboard() *clipEngine {
	ports := real.NewPorts()
	ports.Paster = real.NewPaster(ports.Helper, p.waitPaletteHidden)
	logger, err := logx.New(ports.FS, ports.Clock, configDir(), logx.Options{FileName: clipLogFileName, Stderr: os.Stderr})
	if err != nil {
		logger, _ = logx.New(nil, ports.Clock, "", logx.Options{Stderr: os.Stderr})
		logger.Error("clipboard.log.open.failed", logx.F{"error": err.Error()})
	}
	e := newClipEngine(clipDeps{
		ports:   ports,
		dir:     configDir(),
		limit:   p.S.ClipLimit,
		redraw:  p.redraw,
		toast:   p.toastFromBackground,
		log:     logger,
		noLinks: p.S.ClipNoLinks,
	})
	e.setEnabled(!p.S.ClipOff)
	return e
}
