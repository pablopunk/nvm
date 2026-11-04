package app

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/search"
)

// fallbackFixtureTitle is the title the acme.fallback fixture gives its query fallback for query.
func fallbackFixtureTitle(query string) string {
	return `Search the fixture for "` + query + `"`
}

// paletteWithFallbackFixture is the golden palette whose extension source is the running acme.fallback fixture.
func paletteWithFallbackFixture(t *testing.T) *App {
	t.Helper()
	m := newFixtureManager(t, "acme.fallback")
	waitForState(t, m, "acme.fallback", supervisor.StateRunning)
	p := goldenPalette(t)
	p.exts.mgr.Store(m)
	return p
}

func TestQueryFallbackIsOfferedForAnyQueryInTheFallbackBand(t *testing.T) {
	p := paletteWithFallbackFixture(t)
	for _, query := range []string{"apps", "zzqx"} {
		row := findRowTitled(t, collectExtensionRows(t, p, query), fallbackFixtureTitle(query))
		if row.Score != scoreQueryFallback {
			t.Fatalf("fallback for %q scores %d, want the fallback band %d", query, row.Score, scoreQueryFallback)
		}
	}
}

func TestQueryFallbackBandSitsBetweenSubstringAndLetterMatches(t *testing.T) {
	if search.Score("Keyboard Shortcuts", "shortcut") <= scoreQueryFallback {
		t.Fatalf("a substring match must outrank the query-fallback band (%d)", scoreQueryFallback)
	}
	if loose := search.Score("Rename Screenshots by Date and Location", "rnm scrnshts"); loose <= 0 || loose >= scoreQueryFallback {
		t.Fatalf("a loose letter match scores %d, want a match below the query-fallback band (%d)", loose, scoreQueryFallback)
	}
}

func TestQueryFallbackRanksBelowCommandMatchesAndAboveTheWebSearchRow(t *testing.T) {
	p := paletteWithFallbackFixture(t)
	rows := rootResultsForTable(p, "apps")
	command, fallback, web := indexOfItem(rows, "Force Quit Apps"), indexOfItem(rows, fallbackFixtureTitle("apps")), indexOfItem(rows, `Search the web for "apps"`)
	if command < 0 || fallback < 0 || web < 0 {
		t.Fatalf("rows missing: command %d, fallback %d, web %d", command, fallback, web)
	}
	if !(command < fallback && fallback < web) {
		t.Fatalf("order command %d, fallback %d, web %d; want command, fallback, web", command, fallback, web)
	}
}

func TestQueryFallbackIgnoresTheDeclaredScore(t *testing.T) {
	p := paletteWithFallbackFixture(t)
	row := findRowTitled(t, collectExtensionRows(t, p, "zzqx"), fallbackFixtureTitle("zzqx"))
	if row.Score == 990 {
		t.Fatalf("fallback kept its declared score 990 instead of the fallback band")
	}
}

func TestQueryFallbackNeverShowsAtTheEmptyRoot(t *testing.T) {
	p := paletteWithFallbackFixture(t)
	rows := rootResultsForTable(p, "")
	for _, row := range rows {
		if strings.HasPrefix(row.Title, "Search the fixture for") {
			t.Fatalf("the empty root shows a query fallback as %q", row.Title)
		}
	}
	root := findRowTitled(t, rows, "Fallback root")
	if root.Score == scoreQueryFallback {
		t.Fatalf("a RootItems row took the fallback band; RootItems must ignore QueryFallback")
	}
}
