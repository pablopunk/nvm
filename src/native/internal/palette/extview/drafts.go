package extview

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// defaultDraftDebounce is how long an editor waits after the last edit before it writes the draft, as the notes
// editor does.
const defaultDraftDebounce = 400 * time.Millisecond

// DraftStore keeps the unsaved text of editor views on the host, in each extension's own store. Writes return only
// once they are on disk, so a draft the editor reports as saved survives the process being killed.
type DraftStore interface {
	LoadDraft(extID, key string) (protocol.DraftLookup, error)
	SaveDraft(extID string, args protocol.DraftSaveArgs) error
	DiscardDraft(extID, key string) error
}
