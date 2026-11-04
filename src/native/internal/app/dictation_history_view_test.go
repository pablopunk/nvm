package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/palette"
)

func TestHistoryListsTranscriptsNewestFirstAndDeletesOne(t *testing.T) {
	p := &App{userReadOnly: true}
	p.rememberDictation("first words")
	p.rememberDictation("second words")
	view := p.dictationHistoryView()

	rows := view.Search("")
	if len(rows) != 3 || rows[0].Title != "second words" || rows[2].Title != "Clear History" {
		t.Fatalf("rows = %+v, want two transcripts newest first and the clear row", rows)
	}
	if got := view.Search("FIRST"); len(got) != 1 || got[0].Title != "first words" {
		t.Fatalf("search FIRST = %+v, want the one matching transcript without the clear row", got)
	}

	p.forgetTranscript(p.dictationTranscripts().Entries()[0].ID)
	if rows := view.Search(""); len(rows) != 2 || rows[0].Title != "first words" {
		t.Fatalf("rows after a delete = %+v, want the remaining transcript", rows)
	}
}

func TestClearHistoryEmptiesTheTranscripts(t *testing.T) {
	p := &App{userReadOnly: true}
	p.rememberDictation("gone soon")
	p.clearTranscripts()
	if got := p.dictationTranscripts().Entries(); len(got) != 0 {
		t.Fatalf("entries after clear = %+v", got)
	}
}

func TestTheTranscriptRowOpensItsPreview(t *testing.T) {
	p := &App{userReadOnly: true}
	p.rememberDictation("the whole transcript")
	host := &recordingHost{}
	p.dictationHistoryView().Search("")[0].Actions[0].Run(host)

	if len(host.pushed) != 1 {
		t.Fatalf("pushed %d views, want the preview", len(host.pushed))
	}
	preview, ok := host.pushed[0].(*palette.View)
	if !ok || preview.Title != "Dictation" || preview.OnSubmit == nil || preview.Body == nil {
		t.Fatalf("pushed = %#v, want the transcript preview view", host.pushed[0])
	}
}
