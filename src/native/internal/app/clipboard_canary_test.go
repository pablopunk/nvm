package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
)

// canaryText is a password a password manager would copy with the concealed marker. It must never reach the
// history, the saved file, the log or a toast.
const canaryText = "canary-secret-123"

// TestConcealedCopyNeverReachesHistoryFileLogOrToast runs the real watcher over the fake clipboard. A concealed
// copy is refused before its text is read; a plain copy after it is recorded, so the watcher is not simply off.
func TestConcealedCopyNeverReachesHistoryFileLogOrToast(t *testing.T) {
	f := newClipFixture(t, clipDefaultLimit)
	f.set.Clipboard.WriteText("copied before the watcher starts")
	f.engine.setEnabled(true)
	f.set.Clock.AwaitTimers(1)

	f.set.Clipboard.WriteConcealedText(canaryText)
	f.set.Clock.Advance(time.Second)
	f.set.Clock.AwaitTimers(1)

	if n := f.engine.history.Len(); n != 0 {
		t.Fatalf("history holds %d entries after a concealed copy", n)
	}
	if found := f.engine.history.Find(clipboard.Query{Text: "canary"}); len(found) != 0 {
		t.Fatalf("search finds the concealed copy: %+v", found)
	}
	select {
	case n := <-f.store.Notices():
		t.Fatalf("the store reported a notice for the concealed copy: %+v", n)
	default:
	}

	f.set.Clipboard.WriteText("plain copy after the secret")
	f.set.Clock.Advance(time.Second)
	f.set.Clock.AwaitTimers(1)
	if got := f.engine.history.Find(clipboard.Query{}); len(got) != 1 || got[0].Text != "plain copy after the secret" {
		t.Fatalf("the plain copy after the concealed one was not recorded: %+v", got)
	}

	f.engine.flush()
	saved, err := f.set.FS.ReadFile(clipFixtureDir + "/" + clipboard.SnapshotDocument)
	if err != nil {
		t.Fatalf("the history was not saved: %v", err)
	}
	if strings.Contains(string(saved), "canary") || !strings.Contains(string(saved), "plain copy") {
		t.Fatalf("saved history = %s", saved)
	}
	for _, entry := range f.logger.Recent(1000) {
		if text := fmt.Sprintf("%+v", entry); strings.Contains(text, "canary") {
			t.Fatalf("the log mentions the concealed copy: %s", text)
		}
	}
}

// TestConcealedCopyIsNotShownInTheHistoryView: the history view, which the user opens, never lists it.
func TestConcealedCopyIsNotShownInTheHistoryView(t *testing.T) {
	f := newClipFixture(t, clipDefaultLimit)
	f.set.Clipboard.WriteConcealedText(canaryText)
	f.engine.setEnabled(true)
	f.set.Clock.AwaitTimers(1)
	f.set.Clock.Advance(time.Second)
	f.set.Clock.AwaitTimers(1)
	p := goldenPalette(t)
	p.clip = f.engine
	goldenOpen(p, clipboardBuiltInView(t, f.engine))
	goldenQuery(p, "canary")
	if rows := p.Results(); len(rows) != 0 {
		t.Fatalf("the view shows the concealed copy: %v", rowTitles(p))
	}
}
