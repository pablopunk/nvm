package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// clipFixtureDir is the config folder of a fixture engine. Nothing is written outside the fake file system.
const clipFixtureDir = "/config/nvm-native"

// clipFixture is a clipboard engine over fake ports: the scripted clipboard, file system, clock, HTTP and resolver
// of platform/fake, the requests the engine makes of the UI thread, the toasts it shows and its log.
type clipFixture struct {
	engine  *clipEngine
	set     *fake.Set
	updates chan func()
	toasts  chan string
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
	f := &clipFixture{set: set, updates: make(chan func(), 64), toasts: make(chan string, 16), logger: logger}
	f.engine = newClipEngine(clipDeps{
		ports:      set.Ports(),
		dir:        clipFixtureDir,
		limit:      limit,
		redraw:     func(fn func()) { trySend(f.updates, fn) },
		toast:      func(msg string) { trySend(f.toasts, msg) },
		log:        logger,
		storeClock: storeClock,
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

// awaitUpdate runs the next request the engine made of the UI thread, failing the test when none comes.
func (f *clipFixture) awaitUpdate(tb testing.TB) {
	tb.Helper()
	select {
	case fn := <-f.updates:
		fn()
	case <-time.After(watchdog):
		tb.Fatal("the engine never asked the UI thread for an update")
	}
}

// awaitToast returns the next toast the engine showed, failing the test when none comes.
func (f *clipFixture) awaitToast(tb testing.TB) string {
	tb.Helper()
	select {
	case msg := <-f.toasts:
		return msg
	case <-time.After(watchdog):
		tb.Fatal("the engine never showed a toast")
	}
	return ""
}

// watchdog bounds a wait for an event in the fixtures. It never stands in for one: waits end at the event.
const watchdog = 10 * time.Second

// goldenClipboard is the clipboard history the goldens show: a pinned note, a copied command, a picture and a
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
	shot := goldenBitmap(t, 640, 360, goldenPictureFill)
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
	_, err := h.SetPinned("e-notes", true)
	must(err)
	must(h.Remember(clipboard.Entry{ID: "e-rebase", Kind: clipboard.KindText, Text: "git rebase -i HEAD~3", CreatedAt: goldenNow.Add(-3 * time.Minute)}))
	f.engine.pics.seed(goldenClipboardImageName, shot)
}

// goldenPictureFill is the colour of the golden picture.
var goldenPictureFill = color.NRGBA{R: 96, G: 128, B: 160, A: 255}

// watchdogTimer returns a channel that fires when the watchdog runs out.
func watchdogTimer() <-chan time.Time {
	return time.After(watchdog)
}

// fakeResponse is a scripted answer of the fake public HTTP client with the given status and a short body.
func fakeResponse(status int) fake.Response {
	return fake.Response{Status: status, Body: []byte("gone")}
}

// pngBytes encodes a w by h picture as PNG, for tests that store pictures.
func pngBytes(t testing.TB, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

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
