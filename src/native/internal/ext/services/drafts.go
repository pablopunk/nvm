package services

import (
	"encoding/json"
	"errors"
	"slices"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

const draftsFile = "drafts.json"

// Draft quotas per extension. A draft is unsaved text for one editor, so the bounds are small.
const (
	maxDrafts          = 64
	maxDraftKeyBytes   = 128
	maxDraftVersionLen = 64
	maxDraftBytes      = 256 << 10
)

// draftsDoc is the file behind Drafts: one entry per editor key.
type draftsDoc struct {
	Drafts []protocol.DraftRecord `json:"drafts"`
}

func newDraftsDoc() draftsDoc {
	return draftsDoc{Drafts: []protocol.DraftRecord{}}
}

// serveDrafts answers the drafts service. Each write is flushed before the call returns, so a draft the editor
// reports as saved survives the process dying right after.
func (h *Handler) serveDrafts(call protocol.ServiceCall) (any, error) {
	docs, err := h.openedDocuments()
	if err != nil {
		return nil, err
	}
	switch call.Op {
	case protocol.OpGet:
		return h.draftGet(docs, call.Args)
	case protocol.OpSave:
		return nil, h.draftSave(docs, call.Args)
	case protocol.OpCommit:
		return h.draftCommit(docs, call.Args)
	case protocol.OpDiscard:
		return nil, h.draftDiscard(docs, call.Args)
	}
	return nil, invalid("drafts has no operation %q", call.Op)
}

func (h *Handler) draftGet(docs *documents, args json.RawMessage) (protocol.DraftLookup, error) {
	var request protocol.KeyArgs
	if err := decodeArgs(args, &request); err != nil {
		return protocol.DraftLookup{}, err
	}
	if err := validDraftKey(request.Key); err != nil {
		return protocol.DraftLookup{}, err
	}
	doc, err := docs.drafts.Load()
	if err != nil {
		return protocol.DraftLookup{}, unavailable("the extension's drafts cannot be read: %v", err)
	}
	if i := slices.IndexFunc(doc.Drafts, sameDraftKey(request.Key)); i >= 0 {
		return protocol.DraftLookup{Found: true, Draft: doc.Drafts[i]}, nil
	}
	return protocol.DraftLookup{}, nil
}

func (h *Handler) draftSave(docs *documents, args json.RawMessage) error {
	var request protocol.DraftSaveArgs
	if err := decodeArgs(args, &request); err != nil {
		return err
	}
	if err := validDraftKey(request.Key); err != nil {
		return err
	}
	if len(request.Version) > maxDraftVersionLen {
		return invalid("a draft version is at most %d bytes", maxDraftVersionLen)
	}
	if len(request.Content) > maxDraftBytes {
		return quota(errDraftTooLarge)
	}
	record := protocol.DraftRecord{
		Key: request.Key, Version: request.Version, Content: request.Content,
		SavedAtMs: h.deps.Clock.Now().UnixMilli(),
	}
	return h.writeDrafts(docs, func(doc *draftsDoc) error {
		if i := slices.IndexFunc(doc.Drafts, sameDraftKey(record.Key)); i >= 0 {
			doc.Drafts[i] = record
			return nil
		}
		if len(doc.Drafts) >= maxDrafts {
			return quota(errTooManyDrafts)
		}
		doc.Drafts = append(doc.Drafts, record)
		return nil
	})
}

// draftCommit removes the draft of a key once the extension has persisted its record, and reports whether a draft
// was there. Version is the version the record now has; it is validated and otherwise unused.
func (h *Handler) draftCommit(docs *documents, args json.RawMessage) (bool, error) {
	var request protocol.DraftCommitArgs
	if err := decodeArgs(args, &request); err != nil {
		return false, err
	}
	if err := validDraftKey(request.Key); err != nil {
		return false, err
	}
	removed := false
	err := h.writeDrafts(docs, func(doc *draftsDoc) error {
		before := len(doc.Drafts)
		doc.Drafts = slices.DeleteFunc(doc.Drafts, sameDraftKey(request.Key))
		removed = len(doc.Drafts) != before
		return nil
	})
	return removed, err
}

func (h *Handler) draftDiscard(docs *documents, args json.RawMessage) error {
	var request protocol.KeyArgs
	if err := decodeArgs(args, &request); err != nil {
		return err
	}
	if err := validDraftKey(request.Key); err != nil {
		return err
	}
	return h.writeDrafts(docs, func(doc *draftsDoc) error {
		doc.Drafts = slices.DeleteFunc(doc.Drafts, sameDraftKey(request.Key))
		return nil
	})
}

// writeDrafts applies change and flushes the document to disk before it returns. A refused change writes nothing.
func (h *Handler) writeDrafts(docs *documents, change func(doc *draftsDoc) error) error {
	var refused error
	err := docs.drafts.Update(func(doc *draftsDoc) {
		refused = change(doc)
	})
	if err == nil && refused == nil {
		err = docs.drafts.Flush()
	}
	if err != nil {
		return unavailable("the extension's drafts cannot take changes: %v", err)
	}
	return refused
}

func validDraftKey(key string) error {
	if key == "" || len(key) > maxDraftKeyBytes {
		return invalid("a draft key is 1 to %d bytes", maxDraftKeyBytes)
	}
	return nil
}

func sameDraftKey(key string) func(protocol.DraftRecord) bool {
	return func(record protocol.DraftRecord) bool { return record.Key == key }
}

var (
	errDraftTooLarge = errors.New("this draft is larger than 256 KB")
	errTooManyDrafts = errors.New("an extension may keep at most 64 drafts")
)
