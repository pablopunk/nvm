package clipboard

import (
	"context"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Watcher timing. Polling starts fast after a change and backs off to pollMax while the clipboard is idle,
// so an idle app reads the clipboard about once a second. changeDebounce gathers a burst of change
// notifications into one read.
const (
	pollMin        = 150 * time.Millisecond
	pollMax        = time.Second
	changeDebounce = 50 * time.Millisecond
	imageRecheck   = 5 * time.Second
	watcherName    = "clipboard.watcher"
)

// concealedKey is the observed-content key of a concealed copy. It is not an entry ID, so the next ordinary
// copy always counts as a change.
const concealedKey = "concealed"

// WatcherDeps are the ports and collaborators of the watcher. Files may be nil, in which case copied image
// files are kept as file lists. Capabilities decides between change notification and polling.
type WatcherDeps struct {
	Clipboard    platform.Clipboard
	Files        platform.FS
	Capabilities platform.Capabilities
	Clock        platform.Clock
	History      *History
	Images       *ImageStore
	Log          *logx.Logger
}

// Watcher turns clipboard changes into history entries. It captures what is copied after it starts: the
// content on the clipboard at startup becomes the baseline and is never stored. Copies made by the app
// itself are suppressed by Suppress. Concealed copies are dropped before any content is read. The watcher
// runs in one goroutine; Suppress may be called from any goroutine.
type Watcher struct {
	deps   WatcherDeps
	rich   platform.RichClipboard
	notify <-chan struct{}

	mu         sync.Mutex
	suppressed map[string]time.Time

	// lastID is the ID of the last entry captured or suppressed; the same content is never captured twice.
	lastID string
	// lastKey identifies the last observed clipboard content; a change resets the poll backoff.
	lastKey        string
	imageCheckedAt time.Time

	// pollObserver, when set before Run, is called after every poll of the loop. Tests use it to wait for
	// a poll to finish; production code leaves it nil.
	pollObserver func()
}

// NewWatcher returns a watcher for deps. It uses change notification when the capability is supported and
// the clipboard returns a channel, and polling otherwise.
func NewWatcher(deps WatcherDeps) *Watcher {
	w := &Watcher{deps: deps, suppressed: map[string]time.Time{}}
	w.rich, _ = deps.Clipboard.(platform.RichClipboard)
	if deps.Capabilities.Supported(platform.CapabilityClipboardChanged) {
		w.notify = deps.Clipboard.Changed()
	}
	return w
}

// WatchJob describes the watcher as a host job for the jobs layer (WP3.6). The watcher itself is a loop,
// so the job runs it and reports its interval and timeout instead of firing per tick.
type WatchJob struct {
	ID        string
	Owner     string
	Scope     string
	PollEvery time.Duration
	Timeout   time.Duration
}

// Job returns the descriptor of the clipboard watcher job.
func (w *Watcher) Job() WatchJob {
	return WatchJob{ID: "clipboard.poll", Owner: "host", Scope: "clipboard", PollEvery: pollMax, Timeout: 2 * time.Second}
}

// Suppress makes the watcher ignore the next copy of the entry id for SuppressWindow, and treats the content
// as already seen afterwards. The app calls it before it writes an entry back to the clipboard.
func (w *Watcher) Suppress(id string) {
	if id == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.suppressed[id] = w.deps.Clock.Now().Add(SuppressWindow)
}

// Start runs the watcher on its own goroutine until ctx is done and returns a channel that closes when the
// loop has exited. A panic is recovered and reported through safego.
func (w *Watcher) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	safego.Go(watcherName, func(ctx context.Context) {
		defer close(done)
		w.Run(ctx)
	}, safego.WithContext(ctx), safego.WithClock(w.deps.Clock))
	return done
}

// Run captures clipboard changes until ctx is done. It records the baseline first, then takes copies from the
// clipboard feed where the system has one (Wayland), waits for change notifications, or polls with backoff.
func (w *Watcher) Run(ctx context.Context) {
	w.prime()
	if w.runFeed(ctx) {
		return
	}
	if w.notify != nil {
		w.runOnChange(ctx)
		return
	}
	w.runPolling(ctx)
}

// prime records what is on the clipboard now, without capturing it.
func (w *Watcher) prime() {
	state := w.readFormats()
	if state.concealed() {
		w.lastKey = concealedKey
		return
	}
	candidate := w.readCandidate(state, true)
	w.lastID = candidateID(candidate)
	w.lastKey = w.lastID
}

// runOnChange waits for change notifications. After one it waits a short debounce window and drains the
// rest, so a burst of copies is read once.
func (w *Watcher) runOnChange(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.notify:
		}
		select {
		case <-ctx.Done():
			return
		case <-w.deps.Clock.After(changeDebounce):
		}
		drainNotifications(w.notify)
		w.poll(true)
	}
}

// poll runs one pollOnce of the loop and then the test observer, if any.
func (w *Watcher) poll(notified bool) bool {
	changed := w.pollOnce(notified)
	if w.pollObserver != nil {
		w.pollObserver()
	}
	return changed
}

// runPolling reads the clipboard on a backoff schedule: pollMin after a change, then doubling each idle poll
// up to pollMax.
func (w *Watcher) runPolling(ctx context.Context) {
	interval := pollMin
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.deps.Clock.After(interval):
		}
		interval = nextPollInterval(interval, w.poll(false))
	}
}

// nextPollInterval returns the wait before the next poll.
func nextPollInterval(current time.Duration, changed bool) time.Duration {
	if changed {
		return pollMin
	}
	return min(2*current, pollMax)
}

// drainNotifications empties a coalesced notification channel without blocking.
func drainNotifications(ch <-chan struct{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// pollOnce reads the clipboard once and captures the content when it is new. It reports whether the content
// differs from the previous observation, which drives the poll backoff. notified tells whether a change
// notification triggered the read, so image reads are not throttled.
func (w *Watcher) pollOnce(notified bool) bool {
	state := w.readFormats()
	if state.concealed() {
		w.logDropped("concealed")
		return w.observe(concealedKey)
	}
	readImage := state.hasImage() && (notified || w.imageDue())
	if readImage && !notified {
		w.imageCheckedAt = w.deps.Clock.Now()
	}
	candidate := w.readCandidate(state, readImage)
	id := candidateID(candidate)
	changed := w.observe(id)
	w.capture(candidate)
	return changed
}

// capture stores a candidate unless it is nothing, the same content as last time, or suppressed.
func (w *Watcher) capture(candidate *candidate) {
	if candidate == nil || candidate.entry.ID == w.lastID {
		return
	}
	if w.takeSuppression(candidate.entry.ID) {
		w.lastID = candidate.entry.ID
		return
	}
	if err := w.store(candidate); err != nil {
		w.deps.Log.Warn("clipboard.store.failed", logx.F{"error": err.Error()})
		return
	}
	w.lastID = candidate.entry.ID
}

// observe records the observed content key and reports whether it changed.
func (w *Watcher) observe(key string) bool {
	changed := key != w.lastKey
	w.lastKey = key
	return changed
}

// imageDue reports whether the image check throttle allows an image read on a poll.
func (w *Watcher) imageDue() bool {
	return w.deps.Clock.Now().Sub(w.imageCheckedAt) >= imageRecheck
}

// takeSuppression reports whether id is suppressed now, and forgets an expired suppression.
func (w *Watcher) takeSuppression(id string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	until, ok := w.suppressed[id]
	if !ok {
		return false
	}
	if w.deps.Clock.Now().Before(until) {
		return true
	}
	delete(w.suppressed, id)
	return false
}

// store saves the image file of a candidate, when it has one, and adds the entry to the history.
func (w *Watcher) store(c *candidate) error {
	entry := c.entry
	if entry.Kind == KindImage {
		ref, err := w.deps.Images.Save(entry.Hash, c.png)
		if err != nil {
			return err
		}
		entry.ImageRef = ref
		entry.Bytes = int64(len(c.png))
	}
	return w.deps.History.Remember(entry)
}

// candidateID returns the entry ID of a candidate, or "" when there is none.
func candidateID(c *candidate) string {
	if c == nil {
		return ""
	}
	return c.entry.ID
}
