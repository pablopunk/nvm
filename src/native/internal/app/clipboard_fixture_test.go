package app

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

// clipFixtureDir is the local data folder of a fixture engine. Nothing is written outside the fake file system.
const clipFixtureDir = "/config/nvm-native"

// clipFixture is a clipboard engine over fake ports: the scripted clipboard, file system, clock, HTTP and resolver
// of platform/fake, the requests the engine makes of the UI thread, the store it saves into and its log.
type clipFixture struct {
	engine  *clipEngine
	set     *fake.Set
	updates chan func()
	store   *store.Store
	logger  *logx.Logger
}

// newClipFixture returns an engine with the given limit of unpinned entries. Its capabilities are the ones of a
// desktop that can paste pictures and text, without change notification, so the watcher polls on the fake clock.
func newClipFixture(tb testing.TB, limit int) *clipFixture {
	tb.Helper()
	return newSeededClipFixture(tb, limit, nil)
}

// newSeededClipFixture is newClipFixture with the fake file system prepared by seed before the engine opens.
func newSeededClipFixture(tb testing.TB, limit int, seed func(files *fake.FS)) *clipFixture {
	tb.Helper()
	set := fake.NewSet()
	set.Capabilities = platform.NewCapabilities(platform.CapabilityClipboardImage, platform.CapabilityClipboardPaste)
	set.Resolver.Map("nevermind.app", "93.184.216.34")
	if seed != nil {
		seed(set.FS)
	}
	// The store and the log time their own work on a clock nobody advances: their writes happen on Flush, and the
	// watcher's timer is the only one a test moves.
	storeClock := fake.NewClock()
	logger, err := logx.New(set.FS, storeClock, "/log", logx.Options{})
	if err != nil {
		tb.Fatalf("logx.New: %v", err)
	}
	tb.Cleanup(func() { _ = logger.Close() })
	st, err := store.Open(set.FS, storeClock, clipFixtureDir)
	if err != nil {
		tb.Fatalf("store.Open: %v", err)
	}
	tb.Cleanup(func() { _ = st.Close() })
	f := &clipFixture{set: set, updates: make(chan func(), 64), store: st, logger: logger}
	f.engine = newClipEngine(clipDeps{
		ports:  set.Ports(),
		dir:    clipFixtureDir,
		limit:  limit,
		store:  st,
		redraw: func(fn func()) { trySend(f.updates, fn) },
		log:    logger,
	})
	tb.Cleanup(func() { f.engine.setEnabled(false) })
	return f
}

// trySend sends value without blocking; a full channel drops it. Fixtures never wait on the UI side.
func trySend[T any](ch chan T, value T) {
	select {
	case ch <- value:
	default:
	}
}

// awaitNotice returns the next notice the store reported, failing the test when none comes.
func (f *clipFixture) awaitNotice(tb testing.TB) store.Notice {
	tb.Helper()
	select {
	case n := <-f.store.Notices():
		return n
	case <-time.After(watchdog):
		tb.Fatal("the store never reported a notice")
	}
	return store.Notice{}
}

// watchdog bounds a wait for an event in the fixtures. It never stands in for one: waits end at the event.
const watchdog = 10 * time.Second

// goldenClipboard is the clipboard history the goldens show: a note, a copied command, a picture and a
// web address with its saved preview. Nothing is read from disk; the picture is seeded into the cache.
func goldenClipboard(t *testing.T) *clipEngine {
	t.Helper()
	f := newClipFixture(t, clipDefaultLimit)
	seedGoldenClipboard(t, f)
	return f.engine
}

// goldenClipboardImageName is the stored name of the golden picture.
var goldenClipboardImageName = clipboard.ImageName(strings.Repeat("c", clipboard.HashLength))

// goldenClipboardLink is the address of the golden link entry, which has a saved preview.
const goldenClipboardLink = "https://nevermind.app/docs/palette"

// seedGoldenClipboard copies the golden entries into the history, oldest first, so the newest ends on top.
func seedGoldenClipboard(t *testing.T, f *clipFixture) {
	t.Helper()
	h := f.engine.history
	shot := goldenPictureBytes(t, 640, 360, goldenPictureFill)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(h.Remember(clipboard.Entry{ID: clipboard.TextID(goldenClipboardLink), Kind: clipboard.KindText, Text: goldenClipboardLink, CreatedAt: goldenNow.Add(-26 * time.Hour)}))
	must(h.SetPreview(goldenClipboardLink, clipboard.LinkPreview{
		URL: goldenClipboardLink, Title: "Palette docs", Description: "How the command palette is built, one key at a time.",
		SiteName: "Nevermind", FetchedAt: goldenNow.Add(-26 * time.Hour),
	}))
	must(h.Remember(clipboard.Entry{ID: "e-shot", Kind: clipboard.KindImage, ImageRef: goldenClipboardImageName, Width: 640, Height: 360, Bytes: 1, CreatedAt: goldenNow.Add(-40 * time.Minute)}))
	must(h.Remember(clipboard.Entry{ID: "e-notes", Kind: clipboard.KindText, Text: "Meeting notes: ship the design package by Friday and review the goldens with Ana", CreatedAt: goldenNow.Add(-2 * time.Hour)}))
	must(h.Remember(clipboard.Entry{ID: "e-rebase", Kind: clipboard.KindText, Text: "git rebase -i HEAD~3", CreatedAt: goldenNow.Add(-3 * time.Minute)}))
	_, err := f.engine.images.Save(strings.Repeat("c", clipboard.HashLength), shot)
	must(err)
}

// goldenPictureFill is the colour of the golden picture.
var goldenPictureFill = color.NRGBA{R: 96, G: 128, B: 160, A: 255}

// benchClipboard returns an engine whose history holds entries, which are given newest first and all fit.
func benchClipboard(tb testing.TB, entries []clipboard.Entry) *clipEngine {
	tb.Helper()
	f := newClipFixture(tb, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		if err := f.engine.history.Remember(entries[i]); err != nil {
			tb.Fatalf("Remember: %v", err)
		}
	}
	return f.engine
}

// emptyClipboard is an engine with an empty history, for the empty state.
func emptyClipboard(t *testing.T) *clipEngine {
	t.Helper()
	return newClipFixture(t, clipDefaultLimit).engine
}
