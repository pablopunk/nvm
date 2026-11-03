package app

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestUnreadableHistoryIsQuarantinedWithANotice: a clipboard.json that does not parse is moved aside by the store,
// the history starts empty, and the user is told that the old file was moved, without its content.
func TestUnreadableHistoryIsQuarantinedWithANotice(t *testing.T) {
	const garbage = "{not json at all, secret-looking-text"
	f := newSeededClipFixture(t, clipDefaultLimit, func(files *fake.FS) {
		files.Seed(clipFixtureDir+"/clipboard.json", []byte(garbage))
	})
	msg := f.awaitToast(t)
	if !strings.Contains(msg, "moved aside") || !strings.Contains(msg, "clipboard.json") {
		t.Fatalf("notice = %q", msg)
	}
	if strings.Contains(msg, "secret-looking-text") {
		t.Fatalf("the notice quotes the file: %q", msg)
	}
	if n := f.engine.history.Len(); n != 0 {
		t.Fatalf("history holds %d entries from an unreadable file", n)
	}
}
