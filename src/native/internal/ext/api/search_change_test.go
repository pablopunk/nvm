package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOnSearchChangeIsAcceptedOnListAndGridViewsOnly(t *testing.T) {
	ref := &ActionRef{ID: "meaning"}
	if err := (View{Kind: ViewList, Title: "Cats", OnSearchChange: ref}).Validate(); err != nil {
		t.Fatalf("list view: %v", err)
	}
	if err := (View{Kind: ViewGrid, Title: "Emoji", Grid: &GridSpec{Columns: 3}, OnSearchChange: ref}).Validate(); err != nil {
		t.Fatalf("grid view: %v", err)
	}
	err := (View{Kind: ViewPreview, Title: "Note", Preview: &PreviewSpec{}, OnSearchChange: ref}).Validate()
	if err == nil || !strings.Contains(err.Error(), "onSearchChange needs a list or grid view") {
		t.Fatalf("preview view error = %v, want onSearchChange refused", err)
	}
}

func TestSearchResultIdsTravelInTheWirePatch(t *testing.T) {
	data, err := json.Marshal(ViewPatch{SearchResultIds: []string{"cat", "tiger"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"searchResultIds":["cat","tiger"]`) {
		t.Fatalf("patch = %s, want searchResultIds on the wire", data)
	}
	empty, _ := json.Marshal(ViewPatch{})
	if strings.Contains(string(empty), "searchResultIds") {
		t.Fatalf("empty patch = %s, want the field omitted", empty)
	}
}
