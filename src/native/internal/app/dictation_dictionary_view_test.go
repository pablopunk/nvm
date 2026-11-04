package app

import "testing"

func TestDictionaryRowsAddTheTypedTermAndRemoveItsRow(t *testing.T) {
	p := &App{}
	view := p.dictionaryView()
	host := &recordingHost{}

	rows := view.Search("Figma")
	if len(rows) != 1 {
		t.Fatalf("rows for a new term = %d, want the add row", len(rows))
	}
	rows[0].Actions[0].Run(host)
	if got := p.dictationSettings().Dictionary; got != "Figma" {
		t.Fatalf("dictionary after Add = %q, want Figma", got)
	}

	rows = view.Search("")
	if len(rows) != 1 || rows[0].Title != "Figma" {
		t.Fatalf("rows = %+v, want the saved term only", rows)
	}
	rows[0].Actions[0].Run(host)
	if got := p.dictationSettings().Dictionary; got != "" {
		t.Fatalf("dictionary after Remove = %q, want empty", got)
	}
}

func TestCleanupSwitchSaysWhatToDoWhenSignedOut(t *testing.T) {
	p := &App{}
	host := &recordingHost{}
	row := p.cleanupWithAIRow(dictationSettings{})
	row.Actions[0].Run(host)
	if len(host.toasts) != 1 || p.dictationSettings().CleanupWithAI {
		t.Fatalf("toasts = %v, cleanup = %v; want the sign-in notice and no change", host.toasts, p.dictationSettings().CleanupWithAI)
	}
}

func TestScreenTermLimitCyclesThroughTheOfferedSizes(t *testing.T) {
	if got := nextScreenTermLimit(10); got != 30 {
		t.Fatalf("after 10 = %d, want 30", got)
	}
	if got := nextScreenTermLimit(50); got != 10 {
		t.Fatalf("after 50 = %d, want to wrap to 10", got)
	}
}
