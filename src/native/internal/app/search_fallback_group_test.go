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
		group.targets([]string{aichats.ID, websearch.ID, "unrelated"})
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
	group.targets([]string{websearch.ID, "unrelated"})
	group.accept(websearch.ID, []search.Item{{ID: "web", QueryFallback: true}}, rows)
	if len(rows[websearch.ID]) != 1 {
		t.Fatal("available fallback was lost")
	}
}

func TestFallbackGroupFailureReleasesOtherSource(t *testing.T) {
	rows := map[string][]search.Item{}
	var group fallbackGroup
	group.targets([]string{aichats.ID, websearch.ID, "held"})
	group.accept(websearch.ID, []search.Item{{ID: "web", QueryFallback: true}}, rows)
	if len(rows[websearch.ID]) != 0 {
		t.Fatal("published before the eligible counterpart answered")
	}
	group.accept(aichats.ID, nil, rows)
	if len(rows[websearch.ID]) != 1 {
		t.Fatal("failure did not release the available fallback")
	}
}

func TestAvailableFallbackPublishesWhileUnrelatedSourceIsHeld(t *testing.T) {
	rows := map[string][]search.Item{}
	var group fallbackGroup
	group.targets([]string{websearch.ID, "held"})
	answers := make(chan string)
	published := make(chan bool, 1)
	release := make(chan struct{})
	go func() {
		answers <- websearch.ID
		<-release
		answers <- "held"
	}()
	go func() {
		id := <-answers
		group.accept(id, []search.Item{{ID: "web", QueryFallback: true}}, rows)
		published <- len(rows[websearch.ID]) == 1
		<-answers
	}()
	if !<-published {
		t.Fatal("unrelated source held available fallback")
	}
	close(release)
}
