package fake

import (
	"context"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// ClipboardFeed is a platform.ClipboardFeed the test drives by hand. Wire it with Clipboard.UseFeed: the
// clipboard then reports each copy through the feed as the Wayland watcher does, and its own reads stay blind,
// as a background Wayland client's are.
type ClipboardFeed struct {
	mu      sync.Mutex
	copies  chan platform.ClipboardCopy
	err     error
	started bool
	ended   bool
}

var _ platform.ClipboardFeed = (*ClipboardFeed)(nil)

// NewClipboardFeed returns a feed that delivers what Emit sends.
func NewClipboardFeed() *ClipboardFeed {
	return &ClipboardFeed{copies: make(chan platform.ClipboardCopy, 64)}
}

// FailWith makes WatchCopies report err, as a missing watch tool does.
func (f *ClipboardFeed) FailWith(err error) { f.mu.Lock(); f.err = err; f.mu.Unlock() }

// Emit sends one copy.
func (f *ClipboardFeed) Emit(copied platform.ClipboardCopy) { f.copies <- copied }

// End closes the feed, as a watch process that gave up does.
func (f *ClipboardFeed) End() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.ended {
		f.ended = true
		close(f.copies)
	}
}

// Started reports whether a consumer has asked for the feed.
func (f *ClipboardFeed) Started() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.started }

// WatchCopies returns the channel Emit feeds.
func (f *ClipboardFeed) WatchCopies(context.Context) (<-chan platform.ClipboardCopy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = true
	if f.err != nil {
		return nil, f.err
	}
	return f.copies, nil
}

// feedBackedClipboard is a Clipboard whose WatchCopies delegates to a feed.
type feedBackedClipboard struct {
	*Clipboard
	feed *ClipboardFeed
}

// WatchCopies delegates to the feed.
func (c feedBackedClipboard) WatchCopies(ctx context.Context) (<-chan platform.ClipboardCopy, error) {
	return c.feed.WatchCopies(ctx)
}

// WithFeed returns the clipboard as a platform.ClipboardFeed that delivers through feed.
func (c *Clipboard) WithFeed(feed *ClipboardFeed) platform.Clipboard {
	return feedBackedClipboard{Clipboard: c, feed: feed}
}
