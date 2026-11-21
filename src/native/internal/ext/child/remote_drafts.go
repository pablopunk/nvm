package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteDrafts is Drafts backed by the host. The host owns the draft store; the extension only commits or discards.
type remoteDrafts struct {
	link *hostLink
}

func (d remoteDrafts) Commit(key, version string) (bool, error) {
	var removed bool
	err := d.link.Call(protocol.ServiceDrafts, protocol.OpCommit, protocol.DraftCommitArgs{Key: key, Version: version}, &removed)
	return removed, err
}

func (d remoteDrafts) Discard(key string) error {
	return d.link.Call(protocol.ServiceDrafts, protocol.OpDiscard, protocol.KeyArgs{Key: key}, nil)
}
