package learning

import (
	"errors"
	"strings"
)

// Proposal is a validated review instruction. Add IDs are supplied by the host, never by the model.
type Proposal struct {
	Operation   string `json:"operation"`
	ID          string `json:"id,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Text        string `json:"text,omitempty"`
	AppliesWhen string `json:"appliesWhen,omitempty"`
}

// ApplyReview applies one reviewed proposal set under the collection CAS. newID is a host-owned ID source;
// it must return unique nonempty IDs. The caller must serialize deletion with trace invalidation.
func (r *Rules) ApplyReview(expected uint64, proposals []Proposal, newID func() (string, error)) error {
	if len(proposals) > 20 || newID == nil {
		return ErrInvalid
	}
	return r.doc.Commit(func(doc *Document) error {
		if doc.Revision != expected {
			return ErrConflict
		}
		touched := map[string]bool{}
		for _, proposal := range proposals {
			proposal.Text = strings.TrimSpace(proposal.Text)
			proposal.AppliesWhen = strings.TrimSpace(proposal.AppliesWhen)
			switch proposal.Operation {
			case "add":
				if proposal.ID != "" || len(doc.Rules) >= maxRules || len(doc.Tombstones) >= maxTombstones {
					return ErrInvalid
				}
				id, err := newID()
				if err != nil {
					return err
				}
				rule := Rule{ID: id, Kind: proposal.Kind, Text: proposal.Text, AppliesWhen: proposal.AppliesWhen, Revision: 1}
				if !validRule(rule) || touched[id] {
					return ErrInvalid
				}
				for _, old := range doc.Rules {
					if old.ID == id {
						return ErrInvalid
					}
				}
				for _, fingerprint := range doc.Tombstones {
					if fingerprint == r.fingerprint(rule.Text) {
						return ErrInvalid
					}
				}
				touched[id] = true
				doc.Rules = append(doc.Rules, rule)
			case "update", "remove":
				if proposal.ID == "" || touched[proposal.ID] {
					return ErrInvalid
				}
				touched[proposal.ID] = true
				index := -1
				for i, old := range doc.Rules {
					if old.ID == proposal.ID {
						index = i
						break
					}
				}
				if index < 0 {
					return ErrConflict
				}
				old := doc.Rules[index]
				if proposal.Operation == "remove" {
					if proposal.Kind != "" || proposal.Text != "" || proposal.AppliesWhen != "" || len(doc.Tombstones) >= maxTombstones {
						return ErrInvalid
					}
					doc.Tombstones = append(doc.Tombstones, r.fingerprint(old.Text))
					doc.Rules = append(doc.Rules[:index], doc.Rules[index+1:]...)
					continue
				}
				updated := Rule{ID: old.ID, Kind: proposal.Kind, Text: proposal.Text, AppliesWhen: proposal.AppliesWhen, Revision: old.Revision + 1}
				if !validRule(updated) {
					return ErrInvalid
				}
				for _, fingerprint := range doc.Tombstones {
					if fingerprint == r.fingerprint(updated.Text) {
						return ErrInvalid
					}
				}
				doc.Rules[index] = updated
			default:
				return errors.New("learning: unsupported proposal")
			}
		}
		if len(proposals) > 0 {
			doc.Revision++
		}
		return nil
	})
}
