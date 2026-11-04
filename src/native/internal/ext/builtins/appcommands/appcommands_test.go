package appcommands

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestQuitIsFoundByItsTitleAndKeywords(t *testing.T) {
	for _, query := range []string{"quit", "Nevermind", "exit"} {
		items, err := Extension{}.SearchItems(nil, query)
		if err != nil || len(items) != 1 {
			t.Fatalf("SearchItems(%q) = %+v, %v; want Quit Nevermind", query, items, err)
		}
		if p := items[0].Primary; p == nil || p.Kind != api.ActionSystem || p.Op != QuitOp {
			t.Errorf("primary = %+v, want the quit op", p)
		}
	}
}

func TestEmptyQueryListsNothing(t *testing.T) {
	if items, _ := (Extension{}).SearchItems(nil, ""); len(items) != 0 {
		t.Fatalf("items = %+v, want none for the empty root", items)
	}
}
