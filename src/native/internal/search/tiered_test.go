package search

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// tieredLearner serves fixed usage by key.
type tieredLearner struct{ uses map[string]Use }

func (l tieredLearner) Boost(string, string) int { return 0 }
func (tieredLearner) Record(string, string)      {}
func (tieredLearner) LastUsed(string) int64      { return 0 }
func (l tieredLearner) Usage(key, _ string) Use  { return l.uses[key] }

func tieredTitles(t *testing.T, learner Learner, query string, rows ...Item) []string {
	t.Helper()
	c := NewCoordinator(Options{Clock: fake.NewClock(), Synchronous: true, Flat: true, Tiered: true, Learner: learner, Limit: 100})
	snap, _ := c.Query(query, []Provider{NewRowsProvider("p", func(context.Context, string) []Item { return rows })})
	titles := make([]string, len(snap.Items))
	for i, it := range snap.Items {
		titles[i] = it.Title
	}
	return titles
}

func assertTitles(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("rows = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rows = %q, want %q", got, want)
		}
	}
}

// loomRows are the rows of the reported bug.
func loomRows() []Item {
	return []Item{
		{Title: "Log out of Nevermind", Kind: KindCommands, LearnKey: "cmd:logout", Score: 20},
		{Title: "Bottom Half", Kind: KindCommands, Subtitle: "Window", Score: 10},
		{Title: "Notes from the loom call", Kind: KindClipboard, Content: true, LearnKey: "clip:1", Score: 90},
		{Title: "loom.com/share/abc", Kind: KindClipboard, Content: true, Score: 80},
		{Title: "Loom", Kind: KindApplications, LearnKey: "app:loom", Score: 100},
		{Title: `Search the web for "loom"`, Kind: KindSearch, QueryFallback: true, Score: 40},
		{Title: `Automate "loom"`, Kind: KindSearch, QueryFallback: true, Score: 40},
		{Title: "Loom Settings", Kind: KindCommands, Score: 80},
	}
}

func TestExactAppNameLeadsWhateverTheUsage(t *testing.T) {
	learner := tieredLearner{uses: map[string]Use{"cmd:logout": {Count: 500, LastUsed: 0}}}
	now := time.Unix(0, 0)
	learner.uses["cmd:logout"] = Use{Count: 500, LastUsed: now.UnixMilli()}
	got := tieredTitles(t, learner, "loom", loomRows()...)
	assertTitles(t, got,
		"Loom", "Loom Settings", "Bottom Half", "Log out of Nevermind",
		"loom.com/share/abc", "Notes from the loom call",
		`Automate "loom"`, `Search the web for "loom"`)
}

func TestUsageReordersWithinATierButNeverAcrossOne(t *testing.T) {
	rows := func() []Item {
		return []Item{
			{Title: "Terminal Notes", Kind: KindFiles, LearnKey: "file:notes"}, // tier 2, class other
			{Title: "Terminal", Kind: KindApplications, LearnKey: "app:terminal"},
			{Title: "Teletype", Kind: KindCommands, LearnKey: "cmd:teletype"}, // tier 5 for "te"? no: tier 2
			{Title: "My Terminal Helper", Kind: KindFiles, LearnKey: "file:helper"},
		}
	}
	clock := fake.NewClock()
	nowMs := clock.Now().UnixMilli()
	daily := Use{Count: 30, LastUsed: nowMs}
	old := Use{Count: 1, LastUsed: nowMs - 90*24*60*60*1000}

	assertTitles(t, tieredTitles(t, tieredLearner{}, "te", rows()...),
		"Terminal", "Teletype", "Terminal Notes", "My Terminal Helper")
	// Daily use of a file lifts it over a command inside tier 2, but not over an application, and never into tier 1 or out of 3.
	got := tieredTitles(t, tieredLearner{uses: map[string]Use{"file:notes": daily, "file:helper": daily}}, "te", rows()...)
	assertTitles(t, got, "Terminal", "Terminal Notes", "Teletype", "My Terminal Helper")
	// A single old use does not pass the gap.
	got = tieredTitles(t, tieredLearner{uses: map[string]Use{"file:notes": old}}, "te", rows()...)
	assertTitles(t, got, "Terminal", "Teletype", "Terminal Notes", "My Terminal Helper")
}

func TestFrecencyScale(t *testing.T) {
	const day = 24 * 60 * 60 * 1000
	if got := Frecency(0, 0, 100*day); got != 0 {
		t.Errorf("an unused row has frecency %d", got)
	}
	if got := Frecency(7, 100*day, 100*day); got <= ClassGap {
		t.Errorf("a week of daily use is worth %d, which must pass a class (%d)", got, ClassGap)
	}
	if got := Frecency(1, 100*day, 100*day); got >= ClassGap {
		t.Errorf("one use just now is worth %d, which must not pass a class", got)
	}
	if got := Frecency(1, 0, 100*day); got >= ClassGap/10 {
		t.Errorf("one old use is worth %d, want almost nothing", got)
	}
	if recent, older := Frecency(5, 99*day, 100*day), Frecency(5, 50*day, 100*day); recent <= older {
		t.Errorf("frecency does not decay: %d then %d", recent, older)
	}
}

func TestClassOrderWithinATier(t *testing.T) {
	got := tieredTitles(t, nil, "ab",
		Row("abc file", KindFiles), Row("abc setting", KindCommands), Row("abc app", KindApplications),
		Item{Title: "abc ext", Kind: "Extension", Class: ClassExtensionCommand},
	)
	assertTitles(t, got, "abc app", "abc ext", "abc setting", "abc file")
}

// Row is a row of kind.
func Row(title, kind string) Item { return Item{Title: title, Kind: kind} }

func TestTieBreakIsShorterTitleThenAlphabeticalThenID(t *testing.T) {
	got := tieredTitles(t, nil, "ab",
		Item{Title: "abcd", ID: "1"}, Item{Title: "abc", ID: "9"}, Item{Title: "abd", ID: "2"}, Item{Title: "abb", ID: "3"})
	assertTitles(t, got, "abb", "abc", "abd", "abcd")
}

func TestTieBreakByIDWhenTitlesAreEqual(t *testing.T) {
	c := NewCoordinator(Options{Clock: fake.NewClock(), Synchronous: true, Flat: true, Tiered: true})
	snap, _ := c.Query("ab", []Provider{NewRowsProvider("p", func(context.Context, string) []Item {
		return []Item{{Title: "abb", ID: "2"}, {Title: "abb", ID: "1"}}
	})})
	if snap.Items[0].ID != "1" {
		t.Fatalf("first ID = %q, want 1", snap.Items[0].ID)
	}
}

func TestChosenRowLeadsOnlyItsOwnTier(t *testing.T) {
	rows := func() []Item {
		return []Item{
			{Title: "Loom", Kind: KindApplications, LearnKey: "app:loom"},               // tier 1
			{Title: "Studio Loom Helper", Kind: KindFiles, LearnKey: "file:helper"},     // tier 3, class other
			{Title: "Notes about Loom", Kind: KindCommands, LearnKey: "cmd:notes"},      // tier 3, class command
			{Title: "Gloomy", Kind: KindFiles, LearnKey: "file:gloomy"},                 // tier 4
			{Title: "Log out of Nevermind", Kind: KindCommands, LearnKey: "cmd:logout"}, // tier 5: the letters are scattered
		}
	}
	learner := func(uses map[string]Use) Learner { return tieredLearner{uses: uses} }
	exact := Use{Count: 1, Choice: ChoiceExact}
	extends := Use{Count: 1, Choice: ChoiceExtends}

	// No choice: the class decides inside tier 3.
	assertTitles(t, tieredTitles(t, nil, "loom", rows()...),
		"Loom", "Notes about Loom", "Studio Loom Helper", "Gloomy", "Log out of Nevermind")
	// A choice for this query moves a row to the top of its own tier and no further.
	got := tieredTitles(t, learner(map[string]Use{"file:helper": exact, "file:gloomy": exact, "cmd:logout": exact}), "loom", rows()...)
	assertTitles(t, got, "Loom", "Studio Loom Helper", "Notes about Loom", "Gloomy", "Log out of Nevermind")
	// A choice for a shorter query this one extends counts too, and yields to a choice for this very query.
	got = tieredTitles(t, learner(map[string]Use{"file:helper": extends, "cmd:notes": exact}), "loom", rows()...)
	assertTitles(t, got, "Loom", "Notes about Loom", "Studio Loom Helper", "Gloomy", "Log out of Nevermind")
	got = tieredTitles(t, learner(map[string]Use{"file:helper": extends}), "loom", rows()...)
	assertTitles(t, got, "Loom", "Studio Loom Helper", "Notes about Loom", "Gloomy", "Log out of Nevermind")
}

// TestNothingBeatsAnExactMatch is the reported bug: rows chosen for "lo" and "loo", however often and recently, stay
// below the exact name that "loom" is.
func TestNothingBeatsAnExactMatch(t *testing.T) {
	now := time.Unix(0, 0).UnixMilli()
	heavy := func(choice Choice) Use { return Use{Count: 900, LastUsed: now, Choice: choice} }
	learner := tieredLearner{uses: map[string]Use{
		"cmd:login": heavy(ChoiceExtends), "cmd:logout": heavy(ChoiceExact), "file:gloomy": heavy(ChoiceExact),
	}}
	got := tieredTitles(t, learner, "loom",
		Item{Title: "Log in to Nevermind", Kind: KindCommands, LearnKey: "cmd:login"},
		Item{Title: "Log out of Nevermind", Kind: KindCommands, LearnKey: "cmd:logout"},
		Item{Title: "Gloomy", Kind: KindFiles, LearnKey: "file:gloomy"},
		Item{Title: "Loom", Kind: KindApplications, LearnKey: "app:loom"},
	)
	if got[0] != "Loom" {
		t.Fatalf("rows = %q, want Loom first", got)
	}
}

func TestDirectRowsLeadAndFallbacksStayLast(t *testing.T) {
	got := tieredTitles(t, nil, "2+2",
		Item{Title: "Search the web", QueryFallback: true, Score: 40},
		Item{Title: "Calculator", Kind: KindApplications},
		Item{Title: "= 4", Direct: true, Score: 1000},
		Item{Title: "2+2 note", Kind: KindFiles},
	)
	assertTitles(t, got, "= 4", "2+2 note", "Calculator", "Search the web")
}

func TestEmptyQueryKeepsScoreOrder(t *testing.T) {
	got := tieredTitles(t, nil, "", Item{Title: "b", Score: 1}, Item{Title: "a", Score: 2})
	assertTitles(t, got, "a", "b")
}

func TestTitlePrefixBeatsInfixAndContentAcrossKinds(t *testing.T) {
	rows := []Item{
		{ID: "content", Title: "Pulse message", Kind: KindClipboard, Content: true},
		{ID: "infix", Title: "Heart Pulse Monitor", Kind: KindFiles},
		{ID: "prefix", Title: "Pulse", Kind: KindApplications},
	}
	rankTiered(rows, newQueryMatcher("pul"), "pul", nil, 0)
	slices.SortStableFunc(rows, compareRows)
	if rows[0].ID != "prefix" || rows[1].ID != "infix" || rows[2].ID != "content" {
		t.Fatalf("ranked = %v", rowIDs(rows))
	}
}
