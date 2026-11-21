package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/store"
)

// quarantineToast is the one-line message for a document the store moved aside. It names the file and what the
// user lost, never the file's content. Every feature drains the same notices channel, so the message is chosen by
// the file, not by the reader.
func quarantineToast(n store.Notice) string {
	name := filepath.Base(n.Path)
	switch {
	case strings.HasPrefix(name, clipboard.SnapshotDocument):
		return fmt.Sprintf("Clipboard history could not be read and was moved aside: %s", n.Path)
	case strings.HasPrefix(name, "usage.json"), strings.HasPrefix(name, "queries.json"):
		return fmt.Sprintf("Saved rankings could not be read and were moved to %s", n.Path)
	case strings.HasPrefix(name, "notes.json"), strings.HasPrefix(name, "drafts.json"), strings.HasPrefix(name, "snippets.json"):
		return fmt.Sprintf("Saved notes or snippets could not be read and were moved to %s", n.Path)
	}
	return fmt.Sprintf("Saved data could not be read and was moved to %s", n.Path)
}
