package app

import (
	"strings"
	"testing"

	appfeat "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/search/usage"
)

// loomFixture is the root with a Loom app, a heavily used "Log out of Nevermind" command and two copies that mention loom.
func loomFixture(t *testing.T) *App {
	t.Helper()
	p := rootTableFixtureWithApps(t, []appfeat.App{{Name: "Loom", ID: "com.loom.desktop", Path: "/usr/share/applications/loom.desktop"}})
	for _, text := range []string{"Notes from the loom call with the design team", "https://www.loom.com/share/abc123"} {
		if err := p.clip.history.Remember(clipboard.Entry{ID: clipboard.TextID(text), Kind: clipboard.KindText, Text: text, CreatedAt: goldenNow}); err != nil {
			t.Fatal(err)
		}
	}
	learn := usage.NewMemory(fake.NewClock())
	for range 40 {
		learn.Record("cmd:Log out of Nevermind", "")
	}
	return withLearner(p, learn)
}

// TestLoomIsFirstAndCopiesFollowEveryNameMatch is the reported bug: an exact app name leads, however much a command is
// used, and the copies that only mention the query follow every row whose name matches.
func TestLoomIsFirstAndCopiesFollowEveryNameMatch(t *testing.T) {
	items := rootResultsForTable(loomFixture(t), "loom")
	if len(items) == 0 || items[0].Title != "Loom" || items[0].MatchTier() != search.TierExact {
		t.Fatalf("first row = %q, want the Loom application: %v", firstTitle(items), rowTitles2(items))
	}
	lastName, firstCopy := -1, len(items)
	for i, it := range items {
		switch {
		case it.Kind == search.KindClipboard:
			firstCopy = min(firstCopy, i)
			if it.MatchTier() != search.TierContent {
				t.Errorf("copy %q is in tier %d, want the content tier", it.Title, it.MatchTier())
			}
		case it.MatchTier() < search.TierContent:
			lastName = i
		}
	}
	if firstCopy == len(items) {
		t.Fatalf("no copy was listed: %v", rowTitles2(items))
	}
	if lastName > firstCopy {
		t.Errorf("a name match (row %d) follows a copy (row %d): %v", lastName, firstCopy, rowTitles2(items))
	}
	for _, loose := range []string{"Emoji & Symbols", "Bottom Half", "Bottom Left Quarter", "Bottom Left Sixth", "Report a Problem"} {
		if indexOfItem(items, loose) >= 0 {
			t.Errorf("%q matches only by scattered letters inside words and must not be listed", loose)
		}
	}
	if n := len(items); n < 2 || !strings.HasPrefix(items[n-1].Title, "Search the web") && !strings.HasPrefix(items[n-1].Title, "Automate") {
		t.Errorf("the query fallbacks are not last: %v", rowTitles2(items))
	}
}

func rowTitles2(items []search.Item) []string {
	titles := make([]string, len(items))
	for i, it := range items {
		titles[i] = it.Title
	}
	return titles
}
