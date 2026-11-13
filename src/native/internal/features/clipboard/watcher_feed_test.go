package clipboard

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// feedHarness is a harness whose clipboard cannot be read (a Wayland client in the background) and reports every
// copy through a feed instead.
type feedHarness struct {
	*harness
	feed    *fake.ClipboardFeed
	watcher *Watcher
}

func newFeedHarness(t *testing.T) *feedHarness {
	t.Helper()
	h := newHarness(t)
	feed := fake.NewClipboardFeed()
	watcher := NewWatcher(WatcherDeps{
		Clipboard:    h.clip.WithFeed(feed),
		Files:        h.files,
		Capabilities: h.capsPoll,
		Clock:        h.clock,
		History:      h.history,
		Images:       h.images,
		Log:          h.log,
	})
	h.actions = NewActions(h.clip, h.paster, h.images, h.history, watcher)
	return &feedHarness{harness: h, feed: feed, watcher: watcher}
}

// drain runs the feed until the test ends it, so every emitted copy has been handled when it returns.
func (h *feedHarness) drain() bool {
	h.feed.End()
	return h.watcher.runFeed(context.Background())
}

func textCopy(text string) platform.ClipboardCopy { return platform.ClipboardCopy{Text: text} }

// TestFeedRecordsEveryCopyOfABurst: several copies in a row, text and images, are all recorded while the clipboard
// itself is unreadable, and the content that was there at startup is not.
func TestFeedRecordsEveryCopyOfABurst(t *testing.T) {
	h := newFeedHarness(t)
	h.feed.Emit(platform.ClipboardCopy{Text: "from before the app started", Initial: true})
	h.feed.Emit(textCopy("one"))
	h.feed.Emit(textCopy("two"))
	h.feed.Emit(platform.ClipboardCopy{PNG: pngSolid(t, 4, 3, red)})
	h.feed.Emit(textCopy("three"))
	h.feed.Emit(platform.ClipboardCopy{PNG: pngSolid(t, 2, 2, blue)})
	if h.drain() {
		t.Fatal("runFeed reported a clean shutdown after the feed closed on its own")
	}
	if got, want := h.history.Len(), 5; got != want {
		t.Fatalf("Len = %d, want %d", got, want)
	}
	texts := h.history.Find(Query{Kinds: []Kind{KindText}})
	if len(texts) != 3 || texts[0].Text != "three" || texts[1].Text != "two" || texts[2].Text != "one" {
		t.Fatalf("text entries = %+v, want three, two, one", texts)
	}
	if got := len(h.history.Find(Query{Kinds: []Kind{KindImage}})); got != 2 {
		t.Fatalf("images recorded = %d, want 2", got)
	}
	if got := imageFileCount(t, h.harness); got != 2 {
		t.Fatalf("image files = %d, want 2", got)
	}
}

func TestFeedBaselineIsRememberedNotStored(t *testing.T) {
	h := newFeedHarness(t)
	h.feed.Emit(platform.ClipboardCopy{Text: "baseline", Initial: true})
	h.feed.Emit(textCopy("baseline"))
	h.drain()
	if h.history.Len() != 0 {
		t.Fatalf("Len = %d: the baseline, copied again unchanged, was recorded", h.history.Len())
	}
}

func TestFeedDropsConcealedCopiesAndKeepsTheNext(t *testing.T) {
	h := newFeedHarness(t)
	h.feed.Emit(platform.ClipboardCopy{Concealed: true})
	h.feed.Emit(textCopy("after the password"))
	h.drain()
	if got, want := h.texts(), []string{"after the password"}; !slices.Equal(got, want) {
		t.Fatalf("texts = %v, want %v", got, want)
	}
}

func TestFeedRecordsTheSameContentOnce(t *testing.T) {
	h := newFeedHarness(t)
	h.feed.Emit(textCopy("same"))
	h.feed.Emit(textCopy("same"))
	h.drain()
	if h.history.Len() != 1 {
		t.Fatalf("Len = %d, want 1", h.history.Len())
	}
}

func TestFeedIgnoresBlankAndOversizedText(t *testing.T) {
	h := newFeedHarness(t)
	h.feed.Emit(textCopy("   \n"))
	h.feed.Emit(textCopy(string(make([]byte, MaxCaptureTextBytes+1))))
	h.feed.Emit(platform.ClipboardCopy{PNG: []byte("not a png")})
	h.drain()
	if h.history.Len() != 0 {
		t.Fatalf("Len = %d, want 0", h.history.Len())
	}
}

func TestFeedRecordsCopiedFilesAsFiles(t *testing.T) {
	h := newFeedHarness(t)
	h.feed.Emit(platform.ClipboardCopy{Files: []string{"/home/a/notes.txt", "/home/a/b.txt"}})
	h.drain()
	files := h.history.Find(Query{Kinds: []Kind{KindFiles}})
	if len(files) != 1 || !slices.Equal(files[0].Paths, []string{"/home/a/notes.txt", "/home/a/b.txt"}) {
		t.Fatalf("files entries = %+v", files)
	}
}

// TestFeedIgnoresTheAppsOwnWrites: copying an entry back reaches the feed as a new copy, and is recognized.
func TestFeedIgnoresTheAppsOwnWrites(t *testing.T) {
	h := newFeedHarness(t)
	h.feed.Emit(textCopy("alpha"))
	h.feed.Emit(textCopy("beta"))
	h.feed.End()
	h.watcher.runFeed(context.Background())
	alpha := h.history.Search("alpha")[0]
	if err := h.actions.Copy(alpha); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	again := fake.NewClipboardFeed()
	h.watcher.deps.Clipboard = h.clip.WithFeed(again)
	again.Emit(textCopy("alpha"))
	again.End()
	h.watcher.runFeed(context.Background())
	if got, want := h.texts(), []string{"beta", "alpha"}; !slices.Equal(got, want) {
		t.Fatalf("texts = %v, want %v", got, want)
	}
}

// TestFeedIgnoresThePlainTextOfARichWrite: the feed carries only the text of an HTML copy the app wrote back.
func TestFeedIgnoresThePlainTextOfARichWrite(t *testing.T) {
	h := newFeedHarness(t)
	entry := Entry{ID: HTMLID("<b>bold</b>"), Kind: KindHTML, Text: "bold", Original: "<b>bold</b>"}
	h.mustRemember(entry)
	if err := h.actions.Copy(entry); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	h.feed.Emit(textCopy("bold"))
	h.drain()
	if h.history.Len() != 1 {
		t.Fatalf("Len = %d, want 1: the plain text of the app's own HTML copy was recorded", h.history.Len())
	}
}

// TestFeedUnavailableFallsBackToReading: a missing watch tool is reported once and the watcher reads the clipboard.
func TestFeedUnavailableFallsBackToReading(t *testing.T) {
	h := newFeedHarness(t)
	h.feed.FailWith(errors.New("wl-paste is not installed"))
	if h.watcher.runFeed(context.Background()) {
		t.Fatal("runFeed claimed to run without a feed")
	}
}

func TestClipboardWithoutAFeedIsReadOrPolled(t *testing.T) {
	h := newHarness(t)
	if h.watcher.runFeed(context.Background()) {
		t.Fatal("runFeed ran for a clipboard that has no feed")
	}
}

// TestRunUsesTheFeedWhenTheClipboardHasOne: Run starts the feed and records a copy made while the app reads
// nothing.
func TestRunUsesTheFeedWhenTheClipboardHasOne(t *testing.T) {
	h := newFeedHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	h.feed.Emit(textCopy("copied in the background"))
	done := h.watcher.Start(ctx)
	deadline := time.Now().Add(5 * time.Second)
	for h.history.Len() < 1 && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	cancel()
	<-done
	if got, want := h.texts(), []string{"copied in the background"}; !slices.Equal(got, want) {
		t.Fatalf("texts = %v, want %v", got, want)
	}
}
