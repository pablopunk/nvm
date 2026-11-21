package search

import (
	"reflect"
	"testing"
)

func titleTexts(title string) []string { return []string{title} }

func TestRankScoredPutsAnExactTitleFirst(t *testing.T) {
	rows := []string{"Create Extension with AI", "Reload Extensions"}
	got := RankScored(rows, "reload extensions", titleTexts)
	if want := []string{"Reload Extensions"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("RankScored = %v, want %v", got, want)
	}
}

func TestRankScoredOrdersByBestScoreAndKeepsTiesInOrder(t *testing.T) {
	rows := []string{"Snippets", "Clipboard Snippets", "Snippet", "Other Snippet Tool", "Snippet Notes", "Unrelated"}
	got := RankScored(rows, "snippet", titleTexts)
	want := []string{"Snippet", "Other Snippet Tool", "Snippet Notes", "Snippets", "Clipboard Snippets"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RankScored = %v, want %v", got, want)
	}
}

func TestRankScoredDropsScatteredLettersAndKeepsEveryRowForAnEmptyQuery(t *testing.T) {
	rows := []string{"Reload Extensions", "Create Extension with AI"}
	if got := RankScored(rows, "rlx", titleTexts); len(got) != 0 {
		t.Fatalf("scattered letters matched: %v", got)
	}
	if got := RankScored(rows, "  ", titleTexts); !reflect.DeepEqual(got, rows) {
		t.Fatalf("empty query changed the order: %v", got)
	}
}
