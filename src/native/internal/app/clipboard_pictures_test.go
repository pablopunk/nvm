package app

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// pictureHarness is a picture cache over a stored picture. Its decode is the test's, and its redraws are queued
// for the test to run on its own goroutine, as the UI thread would.
type pictureHarness struct {
	pics    *clipPictures
	name    string
	updates chan func()
}

// newPictureHarness stores one picture and returns a cache whose decode is decode.
func newPictureHarness(t *testing.T, decode func([]byte) (*ui.Bitmap, error)) pictureHarness {
	t.Helper()
	set := fake.NewSet()
	logger, err := logx.New(set.FS, set.Clock, "/log", logx.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	images := clipboard.NewImageStore(clipboard.ImageStoreDeps{Files: set.FS, Clock: set.Clock, Dir: "/pics", Log: logger})
	name, err := images.Save(strings.Repeat("d", clipboard.HashLength), []byte("stored picture bytes"))
	if err != nil {
		t.Fatal(err)
	}
	updates := make(chan func(), 8)
	return pictureHarness{
		pics:    newClipPictures(images, func(fn func()) { updates <- fn }, decode),
		name:    name,
		updates: updates,
	}
}

// TestPictureDecodeNeverRunsOnTheUIThread: asking for a picture returns at once while its decode is still blocked,
// so the decode runs on another goroutine; the decoded picture arrives later through the UI thread.
func TestPictureDecodeNeverRunsOnTheUIThread(t *testing.T) {
	gate := make(chan struct{})
	started := make(chan struct{}, 1)
	h := newPictureHarness(t, func([]byte) (*ui.Bitmap, error) {
		started <- struct{}{}
		<-gate
		return new(ui.Bitmap), nil
	})

	type answer struct {
		state picState
		bm    *ui.Bitmap
	}
	asked := make(chan answer, 1)
	go func() {
		bm, state := h.pics.bitmap(h.name)
		asked <- answer{state: state, bm: bm}
	}()
	select {
	case got := <-asked:
		if got.bm != nil || got.state != picLoading {
			t.Fatalf("bitmap = %v, %v; want nothing yet, loading", got.bm, got.state)
		}
	case <-watchdogTimer():
		t.Fatal("asking for a picture blocked on its decode")
	}
	<-started
	close(gate)
	h.awaitRedraw(t)
	bm, state := h.pics.bitmap(h.name)
	if bm == nil || state != picReady {
		t.Fatalf("after the decode: %v, %v; want the picture", bm, state)
	}
}

// TestPictureDecodeFailureIsRemembered: a picture that cannot be decoded is reported as failed and not decoded
// again.
func TestPictureDecodeFailureIsRemembered(t *testing.T) {
	calls := make(chan struct{}, 4)
	h := newPictureHarness(t, func([]byte) (*ui.Bitmap, error) {
		calls <- struct{}{}
		return nil, errors.New("not a picture")
	})
	if _, state := h.pics.bitmap(h.name); state != picLoading {
		t.Fatalf("first state = %v, want loading", state)
	}
	h.awaitRedraw(t)
	if _, state := h.pics.bitmap(h.name); state != picFailed {
		t.Fatalf("state after failure = %v, want failed", state)
	}
	if _, state := h.pics.bitmap(h.name); state != picFailed || len(calls) != 1 {
		t.Fatalf("the failed picture was decoded again (%d calls)", len(calls))
	}
}

// TestPictureWithoutAnameIsFailed: an entry without a stored picture has nothing to show.
func TestPictureWithoutAnameIsFailed(t *testing.T) {
	h := newPictureHarness(t, func([]byte) (*ui.Bitmap, error) { return new(ui.Bitmap), nil })
	if bm, state := h.pics.bitmap(""); bm != nil || state != picFailed {
		t.Fatalf("empty name = %v, %v", bm, state)
	}
}

// awaitRedraw runs the next update the cache asked the UI thread for.
func (h pictureHarness) awaitRedraw(t *testing.T) {
	t.Helper()
	select {
	case fn := <-h.updates:
		fn()
	case <-watchdogTimer():
		t.Fatal("the picture never reached the UI thread")
	}
}

// TestPreviewPictureIsBoundedBeforeTheUIThread: a picture larger than the preview edge is reduced before it reaches
// the bitmap, so the pane holds a bounded picture.
func TestPreviewPictureIsBoundedBeforeTheUIThread(t *testing.T) {
	big := pngBytes(t, 3000, 1500)
	small, err := clipboard.PreviewPNG(big, clipPreviewEdge)
	if err != nil {
		t.Fatalf("PreviewPNG: %v", err)
	}
	if len(small) == 0 || len(small) >= len(big) {
		t.Fatalf("preview bytes = %d, original %d", len(small), len(big))
	}
	if _, err := decodePicture(big); err != nil {
		t.Fatalf("decodePicture: %v", err)
	}
}

// TestLinkPreviewLoadingState: while the page is being fetched the card says so, and the fetch is started by the
// card, not by the history.
func TestLinkPreviewLoadingState(t *testing.T) {
	f := newClipFixture(t, clipDefaultLimit)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	f.set.PublicHTTP.Handle(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	mustRemember(t, f, goldenClipboardLink)
	p, _ := clipPalette(t, f)
	goldenOpen(p, p.clipboardView())
	chooseRow(t, p, goldenClipboardLink)
	golden.AssertMatrix(t, "clipboard-link-loading", p.View, paletteGoldens)
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	if _, ok := tt.Find(clipPreviewLoading); !ok {
		t.Fatalf("the loading card does not say it is loading")
	}
}

// TestLinkPreviewLoadedState: a preview saved with the history is shown at once and not fetched again.
func TestLinkPreviewLoadedState(t *testing.T) {
	p, _, _ := seededClipPalette(t)
	chooseRow(t, p, goldenClipboardLink)
	golden.AssertMatrix(t, "clipboard-link-ready", p.View, paletteGoldens)
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	if _, ok := tt.Find("Palette docs"); !ok {
		t.Fatal("the loaded card does not show the page title")
	}
}

// TestLinkPreviewFailedState: a page that cannot be fetched is marked unavailable, and the card says so.
func TestLinkPreviewFailedState(t *testing.T) {
	f := newClipFixture(t, clipDefaultLimit)
	f.set.PublicHTTP.Script(fakeResponse(http.StatusInternalServerError))
	mustRemember(t, f, goldenClipboardLink)
	p, _ := clipPalette(t, f)
	goldenOpen(p, p.clipboardView())
	chooseRow(t, p, goldenClipboardLink)
	p.clip.links.Load(goldenClipboardLink)
	f.awaitUpdate(t)
	golden.AssertMatrix(t, "clipboard-link-failed", p.View, paletteGoldens)
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	if _, ok := tt.Find(clipPreviewFailed); !ok {
		t.Fatal("the failed card does not say the preview is unavailable")
	}
}

// mustRemember copies text into the fixture's history with its current time.
func mustRemember(t *testing.T, f *clipFixture, text string) {
	t.Helper()
	if err := f.engine.history.Remember(clipboard.Entry{ID: clipboard.TextID(text), Kind: clipboard.KindText, Text: text, CreatedAt: goldenNow}); err != nil {
		t.Fatal(err)
	}
}

// chooseRow moves the cursor to the row whose title is title.
func chooseRow(t *testing.T, p *App, title string) {
	t.Helper()
	for i, it := range p.Results() {
		if it.Title == title {
			p.Choose(i)
			return
		}
	}
	t.Fatalf("no row %q in %v", title, rowTitles(p))
}
