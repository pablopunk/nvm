package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/builtins/aichats"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/websearch"
	"github.com/pablopunk/nvm/native/internal/search"
	"testing"
)

func TestFallbackGroupPublishesTogetherInEitherArrivalOrder(t *testing.T) {
	for _, first := range []string{aichats.ID, websearch.ID} {
		second := aichats.ID
		if first == second {
			second = websearch.ID
		}
		rows := map[string][]search.Item{}
		var group fallbackGroup
		group.accept(first, []search.Item{{ID: first, QueryFallback: true}, {ID: "ordinary"}}, rows)
		if len(rows[first]) != 1 || rows[first][0].ID != "ordinary" {
			t.Fatalf("early fallback escaped: %+v", rows)
		}
		group.accept(second, []search.Item{{ID: second, QueryFallback: true}}, rows)
		if len(rows[first]) != 2 || len(rows[second]) != 1 {
			t.Fatalf("fallback pair missing: %+v", rows)
		}
	}
}

func TestFallbackGroupFlushesWhenOneProviderIsAbsent(t *testing.T) {
	rows := map[string][]search.Item{}
	var group fallbackGroup
	group.accept(websearch.ID, []search.Item{{ID: "web", QueryFallback: true}}, rows)
	group.flush(rows)
	if len(rows[websearch.ID]) != 1 {
		t.Fatal("available fallback was lost")
	}
}
