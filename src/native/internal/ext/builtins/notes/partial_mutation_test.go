package notes

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

var errAfterMutation = errors.New("follow-up failed")

type failingNotes struct {
	api.Notes
	failList, failRecent bool
}

func (n failingNotes) List() ([]api.Note, error) {
	if n.failList {
		return nil, errAfterMutation
	}
	return n.Notes.List()
}
func (n failingNotes) Recent() (api.Note, error) {
	if n.failRecent {
		return api.Note{}, errAfterMutation
	}
	return n.Notes.Recent()
}

type failingDrafts struct{ api.Drafts }

func (f failingDrafts) Commit(string, string) (bool, error) { return false, errAfterMutation }

type partialContext struct {
	fakeContext
	notesAPI  api.Notes
	draftsAPI api.Drafts
}

func (c partialContext) Notes() api.Notes   { return c.notesAPI }
func (c partialContext) Drafts() api.Drafts { return c.draftsAPI }

func TestPartialMutationsReportChangeAndFailure(t *testing.T) {
	for _, action := range []string{refDeleteFromList, refDeleteShown, refAutosave, api.NoteConflictRestoreRef} {
		t.Run(action, func(t *testing.T) {
			base := newContext(note("n", "old", testNow))
			ctx := partialContext{fakeContext: base, notesAPI: failingNotes{Notes: base.notes, failList: action == refDeleteFromList, failRecent: action == refDeleteShown}, draftsAPI: failingDrafts{Drafts: base.drafts}}
			in := invocation(action, map[string]string{argID: "n", argText: "new"})
			in.EditorContent = "new"
			result, err := New().RunAction(ctx, in)
			if !errors.Is(err, errAfterMutation) || !result.DataChanged {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if action == refDeleteFromList || action == refDeleteShown {
				if len(base.notes.notes) != 0 {
					t.Fatal("delete did not occur")
				}
			} else if base.notes.notes[0].Markdown != "new" {
				t.Fatal("write did not occur")
			}
		})
	}
}
