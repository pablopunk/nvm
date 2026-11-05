package child

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteNotes is the notes the host keeps. Every call is answered by the host's notes store.
type remoteNotes struct {
	link *hostLink
}

func (n remoteNotes) List() ([]api.Note, error) {
	var notes []api.Note
	err := n.link.Call(protocol.ServiceNotes, protocol.OpList, nil, &notes)
	return notes, err
}

func (n remoteNotes) Get(id string) (api.Note, bool, error) {
	var result protocol.NoteGetResult
	err := n.link.Call(protocol.ServiceNotes, protocol.OpGet, protocol.KeyArgs{Key: id}, &result)
	return result.Note, result.Found, err
}

func (n remoteNotes) Create() (api.Note, error) {
	var note api.Note
	err := n.link.Call(protocol.ServiceNotes, protocol.OpCreate, nil, &note)
	return note, err
}

func (n remoteNotes) Recent() (api.Note, error) {
	var note api.Note
	err := n.link.Call(protocol.ServiceNotes, protocol.OpRecent, nil, &note)
	return note, err
}

func (n remoteNotes) Update(id, markdown string, updatedAt time.Time) (api.Note, bool, error) {
	var result protocol.NoteUpdateResult
	err := n.link.Call(protocol.ServiceNotes, protocol.OpUpdate, protocol.NoteUpdateArgs{ID: id, Markdown: markdown, SavedAt: updatedAt}, &result)
	return result.Note, result.Conflict, err
}

func (n remoteNotes) Delete(id string) error {
	return n.link.Call(protocol.ServiceNotes, protocol.OpDelete, protocol.KeyArgs{Key: id}, nil)
}
