package services

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/store"
)

// serveNotes answers the notes service over the Notes built-in's store. Delete asks the user first.
func (h *Handler) serveNotes(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityNotes); err != nil {
		return nil, err
	}
	store := h.notesOrNil()
	if store == nil {
		return nil, unavailable("notes are not available yet")
	}
	switch call.Op {
	case protocol.OpList:
		return noteValues(store.List()), nil
	case protocol.OpGet:
		return h.noteGet(store, call.Args)
	case protocol.OpCreate:
		return h.noteCreate(store)
	case protocol.OpRecent:
		return h.noteRecent(store)
	case protocol.OpUpdate:
		return h.noteUpdate(store, call.Args)
	case protocol.OpDelete:
		return nil, h.noteDelete(ctx, store, call.Args)
	}
	return nil, invalid("notes has no operation %q", call.Op)
}

func (h *Handler) notesOrNil() *notes.Store {
	if h.deps.Notes == nil {
		return nil
	}
	return h.deps.Notes()
}

func (h *Handler) noteGet(store *notes.Store, raw json.RawMessage) (protocol.NoteGetResult, error) {
	var args protocol.KeyArgs
	if err := decodeArgs(raw, &args); err != nil {
		return protocol.NoteGetResult{}, err
	}
	n, found := store.Get(args.Key)
	return protocol.NoteGetResult{Note: noteValue(n), Found: found}, nil
}

func (h *Handler) noteCreate(store *notes.Store) (api.Note, error) {
	n, err := store.Create(h.deps.Clock.Now())
	return noteValue(n), noteError(err)
}

func (h *Handler) noteRecent(store *notes.Store) (api.Note, error) {
	n, err := store.MostRecentOrCreate(h.deps.Clock.Now())
	return noteValue(n), noteError(err)
}

func (h *Handler) noteUpdate(store *notes.Store, raw json.RawMessage) (protocol.NoteUpdateResult, error) {
	var args protocol.NoteUpdateArgs
	if err := decodeArgs(raw, &args); err != nil {
		return protocol.NoteUpdateResult{}, err
	}
	n, conflict, err := store.SaveIfUnchanged(args.ID, args.Markdown, args.SavedAt, h.deps.Clock.Now())
	return protocol.NoteUpdateResult{Note: noteValue(n), Conflict: conflict}, noteError(err)
}

func (h *Handler) noteDelete(ctx context.Context, store *notes.Store, raw json.RawMessage) error {
	if err := requireRun(protocol.RequestMethodOf(ctx), "deleting a note"); err != nil {
		return err
	}
	var args protocol.KeyArgs
	if err := decodeArgs(raw, &args); err != nil {
		return err
	}
	if err := h.confirmDestructive(ctx, "Delete note"); err != nil {
		return err
	}
	return noteError(store.Delete(args.Key))
}

// noteValues converts the store's notes to the API's shape, in the store's order.
func noteValues(list []notes.Note) []api.Note {
	out := make([]api.Note, 0, len(list))
	for _, n := range list {
		out = append(out, noteValue(n))
	}
	return out
}

func noteValue(n notes.Note) api.Note {
	return api.Note{ID: n.ID, Markdown: n.Markdown, UpdatedAt: n.UpdatedAt}
}

// noteError turns the store's refusals into failures the extension can show. A store another instance holds is
// read-only; a missing note and the note limit are the user's to act on.
func noteError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, notes.ErrNotFound):
		return invalid("this note no longer exists")
	case errors.Is(err, notes.ErrLimit):
		return invalid("%s", err.Error())
	case errors.Is(err, store.ErrReadOnly):
		return unavailable("notes are read-only while another Nevermind instance holds them")
	}
	return err
}
