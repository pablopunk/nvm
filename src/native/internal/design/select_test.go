package design

import (
	"slices"
	"testing"
)

// fakeSelectItems records each item the menu adds, and reports the choice of the item whose title is chooseTitle.
type fakeSelectItems struct {
	chooseTitle string
	titles      []string
	checked     []bool
}

func (f *fakeSelectItems) item(title string, checked bool) bool {
	f.titles = append(f.titles, title)
	f.checked = append(f.checked, checked)
	return title == f.chooseTitle
}

// TestSelectMenuChecksOnlyTheSelectedOption checks the menu lists every option and marks the current one.
func TestSelectMenuChecksOnlyTheSelectedOption(t *testing.T) {
	var fake fakeSelectItems
	fillSelectMenu([]string{"Fast", "Smart"}, 1, fake.item)
	if !slices.Equal(fake.titles, []string{"Fast", "Smart"}) {
		t.Fatalf("items = %v, want Fast and Smart in order", fake.titles)
	}
	if !slices.Equal(fake.checked, []bool{false, true}) {
		t.Fatalf("checked = %v, want a check mark on Smart only", fake.checked)
	}
}

// TestSelectMenuReportsTheChosenOption checks the index of the item the user chose is returned.
func TestSelectMenuReportsTheChosenOption(t *testing.T) {
	fake := fakeSelectItems{chooseTitle: "Smart"}
	if got := fillSelectMenu([]string{"Fast", "Smart"}, 0, fake.item); got != 1 {
		t.Fatalf("chosen = %d, want 1 for Smart", got)
	}
}

// TestSelectMenuReportsNoChoiceWhenNoItemIsChosen checks that a menu closed without a choice reports -1.
func TestSelectMenuReportsNoChoiceWhenNoItemIsChosen(t *testing.T) {
	fake := fakeSelectItems{}
	if got := fillSelectMenu([]string{"Fast", "Smart"}, 0, fake.item); got != -1 {
		t.Fatalf("chosen = %d, want -1 when nothing was chosen", got)
	}
}

// TestSelectedTitleIsEmptyWithoutAMatchingOption checks the control shows nothing for an index it does not have.
func TestSelectedTitleIsEmptyWithoutAMatchingOption(t *testing.T) {
	if got := selectedTitle(SelectData{Options: []string{"All"}, Selected: 3}); got != "" {
		t.Fatalf("title = %q, want empty for a missing option", got)
	}
	if got := selectedTitle(SelectData{Options: []string{"All", "Done"}, Selected: 1}); got != "Done" {
		t.Fatalf("title = %q, want Done", got)
	}
}
