package clipboard

import (
	"context"
	"strings"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// runFeed captures copies pushed by the clipboard's feed, which keeps working while the app is in the background
// (a Wayland client cannot read the clipboard without focus). It returns true when it ran until ctx was done.
// It returns false, after logging one warning, when the system needs a feed that is not available or the feed
// ended on its own, and the caller falls back to reading the clipboard.
func (w *Watcher) runFeed(ctx context.Context) bool {
	feed, ok := w.deps.Clipboard.(platform.ClipboardFeed)
	if !ok {
		return false
	}
	copies, err := feed.WatchCopies(ctx)
	if err != nil {
		w.deps.Log.Warn("clipboard.feed.unavailable", logx.F{"error": err.Error()})
		return false
	}
	if copies == nil {
		return false
	}
	for {
		select {
		case <-ctx.Done():
			return true
		case copied, open := <-copies:
			if !open {
				if ctx.Err() != nil {
					return true
				}
				w.deps.Log.Warn("clipboard.feed.stopped", logx.F{})
				return false
			}
			w.captureCopy(copied)
		}
	}
}

// captureCopy applies the capture rules to one copy from the feed. The initial copy is the baseline: it is
// remembered and never stored.
func (w *Watcher) captureCopy(copied platform.ClipboardCopy) {
	if copied.Concealed {
		w.logDropped("concealed")
		w.lastKey = concealedKey
		return
	}
	candidate := w.feedCandidate(copied)
	if copied.Initial {
		w.lastID = candidateID(candidate)
		w.lastKey = w.lastID
		return
	}
	w.observe(candidateID(candidate))
	w.capture(candidate)
}

// feedCandidate describes a copy from the feed, or nil when there is nothing to store.
func (w *Watcher) feedCandidate(copied platform.ClipboardCopy) *candidate {
	if len(copied.Files) > 0 {
		return w.fileCandidate(copied.Files)
	}
	if len(copied.PNG) > 0 {
		return w.feedImageCandidate(copied.PNG)
	}
	text := strings.TrimSpace(copied.Text)
	if text == "" {
		return nil
	}
	if len(text) > MaxCaptureTextBytes {
		w.logDropped("text-too-large")
		return nil
	}
	return &candidate{entry: Entry{ID: TextID(text), Kind: KindText, Text: text}}
}

// feedImageCandidate checks the size and pixels of a PNG from the feed.
func (w *Watcher) feedImageCandidate(png []byte) *candidate {
	if len(png) > MaxCaptureImageBytes {
		w.logDropped("image-too-large")
		return nil
	}
	img, err := decodeLimited(png)
	if err != nil {
		w.logDropped("image-undecodable")
		return nil
	}
	return candidateFromImage(img, png)
}
