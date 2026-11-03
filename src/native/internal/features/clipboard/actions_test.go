package clipboard

import (
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// Cases ported from clipboard-history.test.ts for the copy, paste, read and restore functions.

// orderClipboard records the order of writes into a shared log.
type orderClipboard struct {
	*fake.Clipboard
	log *[]string
}

// WriteText records the write, then delegates.
func (c orderClipboard) WriteText(text string) error {
	*c.log = append(*c.log, "write")
	return c.Clipboard.WriteText(text)
}

// orderPaster records the paste calls into a shared log and runs an optional body inside the call.
type orderPaster struct {
	log  *[]string
	body func()
}

// PasteIntoFrontmost records the start and end of the call.
func (p orderPaster) PasteIntoFrontmost(string) error {
	*p.log = append(*p.log, "paste:start")
	if p.body != nil {
		p.body()
	}
	*p.log = append(*p.log, "paste:end")
	return nil
}

// orderedActions returns actions whose clipboard and paster share one log.
func orderedActions(t *testing.T, h *harness, log *[]string) *Actions {
	t.Helper()
	return NewActions(orderClipboard{Clipboard: h.clip, log: log}, orderPaster{log: log}, h.images, h.history, h.watcher)
}

// TestSaveStateCapturesCurrentClipboard ports 'clipboardSnapshot captures current clipboard state'.
func TestSaveStateCapturesCurrentClipboard(t *testing.T) {
	h := newHarness(t)
	if err := h.clip.WriteRich("snap text", "<b>snap</b>"); err != nil {
		t.Fatal(err)
	}
	state, err := h.actions.SaveState()
	if err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	if state.Text != "snap text" || state.HTML != "<b>snap</b>" || state.PNG != nil {
		t.Fatalf("state = %+v", state)
	}
}

// TestRestoreStateRestoresRichContent ports 'restoreClipboardSnapshot restores state'.
func TestRestoreStateRestoresRichContent(t *testing.T) {
	h := newHarness(t)
	if err := h.actions.RestoreState(ClipboardState{Text: "restored", HTML: "<b>restored</b>"}); err != nil {
		t.Fatalf("RestoreState: %v", err)
	}
	if text, _ := h.clip.ReadText(); text != "restored" {
		t.Fatalf("text = %q", text)
	}
	if html, _ := h.clip.ReadHTML(); html != "<b>restored</b>" {
		t.Fatalf("html = %q", html)
	}
}

// TestRestorePlainTextUsesTextWriter ports 'restoreClipboardSnapshot restores plain text through the platform text
// writer': plain text goes through WriteText, with no markup written.
func TestRestorePlainTextUsesTextWriter(t *testing.T) {
	h := newHarness(t)
	if err := h.actions.RestoreState(ClipboardState{Text: "restored"}); err != nil {
		t.Fatalf("RestoreState: %v", err)
	}
	if text, _ := h.clip.ReadText(); text != "restored" {
		t.Fatalf("text = %q", text)
	}
	if html, _ := h.clip.ReadHTML(); html != "" {
		t.Fatalf("markup written for plain text: %q", html)
	}
}

// TestRestoreEmptyStateClearsClipboard ports 'restoreClipboardSnapshot clears on empty snapshot'.
func TestRestoreEmptyStateClearsClipboard(t *testing.T) {
	h := newHarness(t)
	mustWrite(t, h, "before")
	if err := h.actions.RestoreState(ClipboardState{}); err != nil {
		t.Fatalf("RestoreState: %v", err)
	}
	if text, _ := h.clip.ReadText(); text != "" {
		t.Fatalf("text = %q, want empty", text)
	}
}

// TestRestoreFilesIsRefusedBeforeWriting: a file list cannot be written yet, so the clipboard is left as it was.
func TestRestoreFilesIsRefusedBeforeWriting(t *testing.T) {
	h := newHarness(t)
	mustWrite(t, h, "keep me")
	if err := h.actions.RestoreState(ClipboardState{Files: []string{"/a"}, Text: "x"}); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("RestoreState = %v, want ErrUnsupported", err)
	}
	if text, _ := h.clip.ReadText(); text != "keep me" {
		t.Fatalf("clipboard changed to %q", text)
	}
}

// TestReadContentReturnsText ports 'readDesktopClipboard returns text when clipboard has text'.
func TestReadContentReturnsText(t *testing.T) {
	h := newHarness(t)
	mustWrite(t, h, "hello clipboard")
	content, err := h.actions.ReadContent()
	if err != nil || content.Kind != ContentText || content.Text != "hello clipboard" {
		t.Fatalf("ReadContent = %+v, %v", content, err)
	}
}

// TestReadContentReturnsEmptyWhenEmpty ports 'readDesktopClipboard returns empty when clipboard is empty'.
func TestReadContentReturnsEmptyWhenEmpty(t *testing.T) {
	h := newHarness(t)
	content, err := h.actions.ReadContent()
	if err != nil || content.Kind != ContentEmpty {
		t.Fatalf("ReadContent = %+v, %v", content, err)
	}
}

// TestReadContentPriorityFilesImageHTMLText: files win over an image, an image over HTML, and HTML over text.
func TestReadContentPriorityFilesImageHTMLText(t *testing.T) {
	h := newHarness(t)
	if err := h.clip.WriteRich("text", "<i>html</i>"); err != nil {
		t.Fatal(err)
	}
	if content, _ := h.actions.ReadContent(); content.Kind != ContentHTML || content.HTML != "<i>html</i>" {
		t.Fatalf("html priority: %+v", content)
	}
	if err := h.clip.WriteImagePNG(pngSolid(t, 1, 1, red)); err != nil {
		t.Fatal(err)
	}
	if content, _ := h.actions.ReadContent(); content.Kind != ContentImage {
		t.Fatalf("image priority: %+v", content)
	}
	h.clip.SetFilePaths("/a.txt")
	if content, _ := h.actions.ReadContent(); content.Kind != ContentFiles {
		t.Fatalf("files priority: %+v", content)
	}
}

// TestPasteWaitsForClipboardWriteBeforeDispatch ports 'pasteTextAction waits for clipboard write before
// dispatching paste'. The write finishes before the paste gesture is sent.
func TestPasteWaitsForClipboardWriteBeforeDispatch(t *testing.T) {
	h := newHarness(t)
	var log []string
	actions := orderedActions(t, h, &log)
	if err := actions.Paste(textEntry("😀", h.clock.Now()), ""); err != nil {
		t.Fatalf("Paste: %v", err)
	}
	if len(log) < 2 || log[0] != "write" || log[1] != "paste:start" {
		t.Fatalf("order = %v, want write before paste", log)
	}
}

// TestPasteWaitsForPasteDispatchToFinish ports 'pasteTextAction waits for paste dispatch to finish': Paste returns
// only after the gesture has been sent.
func TestPasteWaitsForPasteDispatchToFinish(t *testing.T) {
	h := newHarness(t)
	var log []string
	actions := NewActions(orderClipboard{Clipboard: h.clip, log: &log}, orderPaster{log: &log}, h.images, h.history, h.watcher)
	if err := actions.Paste(textEntry("😀", h.clock.Now()), ""); err != nil {
		t.Fatalf("Paste: %v", err)
	}
	if got := log[len(log)-1]; got != "paste:end" {
		t.Fatalf("order = %v, want the paste to end before Paste returns", log)
	}
}

// TestCopyWritesText ports 'writeDesktopClipboard writes text'.
func TestCopyWritesText(t *testing.T) {
	h := newHarness(t)
	if err := h.actions.Copy(textEntry("written", h.clock.Now())); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if text, _ := h.clip.ReadText(); text != "written" {
		t.Fatalf("text = %q", text)
	}
}

// TestCopySuppressesTheWatcher ports 'writeDesktopClipboard suppresses history for concealed content': the app's
// own write is registered with the watcher so it is not recorded again.
func TestCopySuppressesTheWatcher(t *testing.T) {
	h := newHarness(t)
	entry := textEntry("secret-ish", h.clock.Now())
	if err := h.actions.Copy(entry); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if !h.watcher.takeSuppression(entry.ID) {
		t.Fatal("the copy was not suppressed")
	}
}

// TestCopyImageWritesTheStoredPNG: an image entry puts its stored picture back on the clipboard.
func TestCopyImageWritesTheStoredPNG(t *testing.T) {
	h := newHarness(t)
	png := pngSolid(t, 3, 2, blue)
	name, err := h.images.Save(storeHash, png)
	if err != nil {
		t.Fatal(err)
	}
	entry := Entry{ID: ImageID(storeHash), Kind: KindImage, ImageRef: name}
	if err := h.actions.Copy(entry); err != nil {
		t.Fatalf("Copy image: %v", err)
	}
	got, err := h.clip.ReadImagePNG()
	if err != nil || string(got) != string(png) {
		t.Fatalf("clipboard image = %d bytes, %v", len(got), err)
	}
}

// TestCopyFilesIsUnsupportedUntilFileWrites: the file-list write is not in the port yet.
func TestCopyFilesIsUnsupportedUntilFileWrites(t *testing.T) {
	h := newHarness(t)
	err := h.actions.Copy(Entry{ID: FilesID([]string{"/a"}), Kind: KindFiles, Paths: []string{"/a"}})
	if !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("Copy(files) = %v, want ErrUnsupported", err)
	}
}

// TestCopyMarksEntryUsed records the use time, which informs ranking later.
func TestCopyMarksEntryUsed(t *testing.T) {
	h := newHarness(t)
	entry := textEntry("used", h.clock.Now())
	h.mustRemember(entry)
	h.clock.Advance(time.Minute)
	if err := h.actions.Copy(entry); err != nil {
		t.Fatal(err)
	}
	got, _ := h.history.Get(entry.ID)
	if !got.UsedAt.Equal(h.clock.Now()) {
		t.Fatalf("UsedAt = %v, want now", got.UsedAt)
	}
}

// TestPasteTargetsTheExpectedApp sends the gesture to the named app and records it.
func TestPasteTargetsTheExpectedApp(t *testing.T) {
	h := newHarness(t)
	if err := h.actions.Paste(textEntry("paste me", h.clock.Now()), "com.apple.TextEdit"); err != nil {
		t.Fatalf("Paste: %v", err)
	}
	if got := h.paster.Requests(); len(got) != 1 || got[0] != "com.apple.TextEdit" {
		t.Fatalf("paster requests = %v", got)
	}
}

// TestPasteReportsUnsupportedPaster: when the OS cannot paste, the error is returned and the entry is not
// marked used.
func TestPasteReportsUnsupportedPaster(t *testing.T) {
	h := newHarness(t)
	entry := textEntry("nope", h.clock.Now())
	h.mustRemember(entry)
	h.paster.FailNext(fake.OpPasteIntoFrontmost, platform.ErrUnsupported)
	if err := h.actions.Paste(entry, ""); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("Paste = %v, want ErrUnsupported", err)
	}
	if got, _ := h.history.Get(entry.ID); !got.UsedAt.IsZero() {
		t.Fatal("a failed paste marked the entry used")
	}
}
