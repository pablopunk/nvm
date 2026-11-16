package search

import (
	"context"
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
		"Loom", "Loom Settings", "Log out of Nevermind",
		"Bottom Half", "loom.com/share/abc", "Notes from the loom call",
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

func TestChosenRowLeadsWhenItStillMatchesByName(t *testing.T) {
	rows := func() []Item {
		return []Item{
			{Title: "Loom", Kind: KindApplications, LearnKey: "app:loom"},
			{Title: "Studio Loom Helper", Kind: KindFiles, LearnKey: "file:helper"},
			{Title: "Log out of Nevermind", Kind: KindCommands, LearnKey: "cmd:logout"},
		}
	}
	chosenFile := func(choice Choice) Learner {
		return tieredLearner{uses: map[string]Use{"file:helper": {Count: 1, Choice: choice}, "cmd:logout": {Count: 1, Choice: ChoiceExact}}}
	}
	// A substring match chosen for this query leads even the exact app.
	assertTitles(t, tieredTitles(t, chosenFile(ChoiceExact), "loom", rows()...), "Studio Loom Helper", "Loom", "Log out of Nevermind")
	// A query it extends counts too, but yields to an exact choice.
	got := tieredTitles(t, tieredLearner{uses: map[string]Use{"file:helper": {Count: 1, Choice: ChoiceExtends}, "app:loom": {Count: 1, Choice: ChoiceExact}}}, "loom", rows()...)
	assertTitles(t, got, "Loom", "Studio Loom Helper", "Log out of Nevermind")
	// A row that only matches by scattered letters, or by content, is never lifted by a choice.
	assertTitles(t, tieredTitles(t, chosenFile(ChoiceNone), "loom", rows()...), "Loom", "Studio Loom Helper", "Log out of Nevermind")
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
