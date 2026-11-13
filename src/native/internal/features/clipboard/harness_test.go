package clipboard

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// imagesDir is where the test image store keeps its files.
const imagesDir = "/data/clipboard-images"

// harness wires the package over the fake ports. The log uses its own clock so its flusher never
// shares timers with the code under test.
type harness struct {
	t        *testing.T
	clock    *fake.Clock
	files    *fake.FS
	clip     *fake.Clipboard
	paster   *fake.Paster
	persist  *MemoryPersistence
	history  *History
	images   *ImageStore
	log      *logx.Logger
	watcher  *Watcher
	actions  *Actions
	capsPoll platform.Capabilities
}

// newHarness returns a harness with an empty history, an image store and a watcher that uses change
// notification.
func newHarness(t *testing.T) *harness {
	t.Helper()
	set := fake.NewSet()
	h := &harness{
		t:       t,
		clock:   set.Clock,
		files:   set.FS,
		clip:    set.Clipboard,
		paster:  set.Paster,
		persist: NewMemoryPersistence(),
		capsPoll: platform.NewCapabilities(
			platform.CapabilityClipboardImage,
			platform.CapabilityClipboardPaste,
		),
	}
	log, err := logx.New(set.FS, fake.NewClock(), "/logs", logx.Options{FlushInterval: time.Hour})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	h.log = log
	history, err := Open(h.persist, h.clock, Retention{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	h.history = history
	h.images = NewImageStore(ImageStoreDeps{Files: set.FS, Clock: h.clock, Dir: imagesDir, Log: log})
	h.watcher = h.newWatcher(set.Capabilities)
	h.actions = NewActions(set.Clipboard, set.Paster, h.images, history, h.watcher)
	return h
}

// newWatcher returns a watcher over the harness history with the given capabilities.
func (h *harness) newWatcher(caps platform.Capabilities) *Watcher {
	return NewWatcher(WatcherDeps{
		Clipboard:    h.clip,
		Files:        h.files,
		Capabilities: caps,
		Clock:        h.clock,
		History:      h.history,
		Images:       h.images,
		Log:          h.log,
	})
}

// pollingWatcher returns a watcher that polls, for tests of the backoff schedule and of the startup rule.
func (h *harness) pollingWatcher() *Watcher {
	return h.newWatcher(h.capsPoll)
}

// texts returns the plain text of every entry, newest first.
func (h *harness) texts() []string {
	entries := h.history.Find(Query{})
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Text)
	}
	return out
}

// mustRemember stores an entry, failing the test on error.
func (h *harness) mustRemember(e Entry) {
	h.t.Helper()
	if err := h.history.Remember(e); err != nil {
		h.t.Fatalf("Remember(%s): %v", e.ID, err)
	}
}

// pngSolid returns a PNG of the given size filled with one color.
func pngSolid(t *testing.T, width, height int, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// textEntry builds a text entry with the trusted ID and the given copy time.
func textEntry(text string, createdAt time.Time) Entry {
	return Entry{ID: TextID(text), Kind: KindText, Text: text, CreatedAt: createdAt}
}

// refsStub is a GCRefs for tests of the sweep rules alone.
type refsStub struct {
	trusted   bool
	names     map[string]bool
	persisted map[string]bool
	persistOK bool
	onLookup  func(name string)
}

func newRefs(names ...string) *refsStub {
	stub := &refsStub{trusted: true, names: map[string]bool{}, persisted: map[string]bool{}, persistOK: true}
	for _, name := range names {
		stub.names[name] = true
	}
	return stub
}

// Trusted reports the stub's trust flag.
func (r *refsStub) Trusted() bool { return r.trusted }

// ImageNames returns the live and persisted names, or an error when the saved copy is unreadable.
func (r *refsStub) ImageNames() (map[string]bool, error) {
	if !r.persistOK {
		return nil, errPersistUnreadable
	}
	out := map[string]bool{}
	for name := range r.names {
		out[name] = true
	}
	for name := range r.persisted {
		out[name] = true
	}
	return out, nil
}

// HasImage reports whether the live names contain the file name, and runs the test's hook first.
func (r *refsStub) HasImage(name string) bool {
	if r.onLookup != nil {
		r.onLookup(name)
	}
	return r.names[name]
}

var errPersistUnreadable = fakeError("saved history unreadable")

// fakeError is a constant error for tests.
type fakeError string

// Error returns the message.
func (e fakeError) Error() string { return string(e) }
