package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/websearch"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/search/usage"
)

// The cases below are the root-level ports of 1.x/electron/search/action-ranking.test.ts. The 1.x fallbacks
// are the Automate and web search rows here, whose fixed scores stand in for the 1.x query-fallback band.

// TestPortedLeadingQueryWordsOutrankFallbacks ports "leading query words in a command outrank generated search
// fallbacks": a command whose first words match the query scores above both fallbacks.
func TestPortedLeadingQueryWordsOutrankFallbacks(t *testing.T) {
	const query = "fix with ai"
	command := search.Score("Fix Selected Text with AI", query)
	if command != 95 {
		t.Fatalf("Score(Fix Selected Text with AI) = %d, want 95", command)
	}
	if command <= scoreAutomate || command <= scoreQueryFallback {
		t.Fatalf("the command (%d) must outrank Automate (%d) and the web search fallback (%d)", command, scoreAutomate, scoreQueryFallback)
	}
}

// TestPortedPartialWordMatchesOutrankFallbacks ports "partial word matches in real commands outrank query
// fallbacks that echo the query": a command containing the query outranks the Automate row that echoes it.
func TestPortedPartialWordMatchesOutrankFallbacks(t *testing.T) {
	command := search.Score("Keyboard Shortcuts", "shortcut")
	if command <= scoreAutomate {
		t.Fatalf("Keyboard Shortcuts (%d) must outrank Automate \"shortcut\" (%d)", command, scoreAutomate)
	}
}

// TestPortedFrequentFallbackUseDoesNotOvertakeAStrongCommand ports "frequent fallback use does not overtake a
// strong command match": the web search row, used a hundred times for the query, still ranks below a command
// that contains the query, and the root shows the command first.
func TestPortedFrequentFallbackUseDoesNotOvertakeAStrongCommand(t *testing.T) {
	p := rootTableFixture(t)
	learn := usage.NewMemory(fake.NewClock())
	for range 100 {
		learn.Record(websearch.SearchLearnKey, "apps")
	}
	p = withLearner(p, learn)

	items := rootResultsForTable(p, "apps")
	command, web := indexOfItem(items, "Force Quit Apps"), indexOfItem(items, `Search the web for "apps"`)
	if command < 0 || web < 0 {
		t.Fatalf("both rows should be listed: command %d, web %d", command, web)
	}
	if command > web {
		t.Fatalf("the frequently used web search (row %d) overtook the command (row %d)", web, command)
	}
	if got := items[web].Score; got >= search.Score("Force Quit Apps", "apps") {
		t.Fatalf("the boosted web search scores %d, which reaches the command's band", got)
	}
}

// TestPortedQueryFallbacksStayAboveLooseMatches ports "query fallbacks stay above loose character matches for
// open-ended prompts": the Automate row outranks a loose match of the prompt's letters.
func TestPortedQueryFallbacksStayAboveLooseMatches(t *testing.T) {
	loose := search.Score("Rename Screenshots by Date and Location", "rnm scrnshts")
	if loose <= 0 {
		t.Fatalf("the loose match should match, got %d", loose)
	}
	if loose >= scoreAutomate {
		t.Fatalf("a loose match (%d) must stay below the Automate row (%d)", loose, scoreAutomate)
	}
}

// indexOfItem returns the position of the row with title, or -1.
func indexOfItem(items []search.Item, title string) int {
	for i, it := range items {
		if it.Title == title {
			return i
		}
	}
	return -1
}
