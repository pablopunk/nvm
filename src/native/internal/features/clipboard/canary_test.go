package clipboard

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// canary is text a password manager would mark as concealed. No test output, file, snapshot or event may
// contain it.
const canary = "CANARY-7f3a9c-concealed-do-not-store"

// TestConcealedCanaryStaysOutOfStorageLogsAndEvents copies a concealed canary through the change loop, then
// proves it appears nowhere: not in the history, the saved snapshot, any file, any log entry at debug level,
// or any change notification. A plain copy after it is recorded, so the watcher still works.
func TestConcealedCanaryStaysOutOfStorageLogsAndEvents(t *testing.T) {
	h := newHarness(t)
	debugLog, err := logx.New(h.files, fake.NewClock(), "/debug-logs", logx.Options{Level: logx.Debug, FlushInterval: time.Hour})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	h.watcher.deps.Log = debugLog

	changes, stop := h.history.Subscribe()
	defer stop()
	polled := make(chan struct{}, 8)
	h.watcher.pollObserver = func() { polled <- struct{}{} }
	h.watcher.prime()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		h.watcher.runOnChange(ctx)
		close(done)
	}()

	h.clip.WriteConcealedText(canary)
	h.clock.AwaitTimers(1)
	h.clock.Advance(changeDebounce)
	<-polled
	h.requireQuiet(changes, "concealed copy was announced")

	mustWrite(t, h, "sentinel after the canary")
	h.clock.AwaitTimers(1)
	h.clock.Advance(changeDebounce)
	<-polled
	<-changes
	cancel()
	<-done

	if got := h.texts(); len(got) != 1 || got[0] != "sentinel after the canary" {
		t.Fatalf("history = %v, want only the sentinel", got)
	}
	assertNoCanary(t, "saved snapshot", mustJSON(t, h.persist.Stored()))
	assertNoCanary(t, "history entries", mustJSON(t, h.history.Find(Query{Limit: 1 << 20})))
	assertNoCanary(t, "log entries", mustJSON(t, debugLog.Recent(1000)))
	if err := debugLog.Flush(); err != nil {
		t.Fatalf("flush log: %v", err)
	}
	for _, path := range h.files.Paths() {
		data, err := h.files.ReadFile(path)
		if err != nil {
			continue
		}
		assertNoCanary(t, "file "+path, data)
	}
}

// TestConcealedCanaryIsNotStoredByPoll checks the poll path alone, with an image copy as well: a concealed image
// writes no file and records no entry.
func TestConcealedCanaryIsNotStoredByPoll(t *testing.T) {
	h := newHarness(t)
	h.clip.WriteConcealedText(canary)
	h.watcher.pollOnce(true)
	h.clip.WriteConcealedImagePNG(pngSolid(t, 2, 2, red))
	h.watcher.pollOnce(true)
	if h.history.Len() != 0 || imageFileCount(t, h) != 0 {
		t.Fatalf("concealed copies stored: entries %d, files %d", h.history.Len(), imageFileCount(t, h))
	}
	if h.persist.Saves() != 0 {
		t.Fatalf("concealed copies saved the history %d times", h.persist.Saves())
	}
}

// requireQuiet fails the test when a change notification is pending.
func (h *harness) requireQuiet(changes <-chan struct{}, why string) {
	h.t.Helper()
	select {
	case <-changes:
		h.t.Fatal(why)
	default:
	}
}

// mustJSON encodes v for a canary search.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

// assertNoCanary fails when data contains the canary text. The name says where it was looked for.
func assertNoCanary(t *testing.T, where string, data []byte) {
	t.Helper()
	if bytes.Contains(data, []byte(canary)) {
		t.Fatalf("the concealed canary reached %s", where)
	}
}
