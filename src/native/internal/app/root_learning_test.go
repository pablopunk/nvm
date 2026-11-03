package app

import (
	"encoding/json"
	"flag"
	"os"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search/usage"
)

// updateLearnedTable rewrites testdata/queries-learned-expected.json. The file is the second table: the root's
// results once the usage below is recorded. Regenerate it only after a deliberate change of learning.
var updateLearnedTable = flag.Bool("update-learned-table", false, "rewrite testdata/queries-learned-expected.json")

// learnedRootQueries are the queries of the second table.
var learnedRootQueries = []string{"", "cal", "term", "set"}

const learnedTableFile = "testdata/queries-learned-expected.json"

// learnedUsageFixture is the table fixture with a learner that holds only the usage below, recorded at the
// fixture clock: Calendar chosen three times for "cal", Terminal chosen three times for "term", and the Settings
// command chosen three times at the root. Nothing else was used, so the usage the golden fixtures seed is gone.
func learnedUsageFixture(t *testing.T) *App {
	t.Helper()
	p := rootTableFixture(t)
	learn := usage.NewMemory(fake.NewClock())
	for range 3 {
		learn.Record("app:org.gnome.Calendar.desktop", "cal")
		learn.Record("app:org.gnome.Terminal.desktop", "term")
		learn.Record("cmd:Settings", "")
	}
	return withLearner(p, learn)
}

// withLearner gives the fixture a learner and rebuilds its palette, which takes the learner when it is built.
func withLearner(p *App, learn *usage.Learner) *App {
	p.learn = learn
	p.newPalette(searchClock, true)
	return p
}

// learnedTable runs the learned queries and encodes the rows the same way the first table does.
func learnedTable(t *testing.T) []byte {
	t.Helper()
	entries := make([]tableEntry, 0, len(learnedRootQueries))
	for _, q := range learnedRootQueries {
		rows := rootTableRows(t, learnedUsageFixture(t), q)
		if rows == nil {
			rows = []rootRow{}
		}
		entries = append(entries, tableEntry{Query: q, Rows: rows})
	}
	encoded, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

// TestRootTableWithRecordedUsageMatchesExpected compares the learned results with the recorded second table.
func TestRootTableWithRecordedUsageMatchesExpected(t *testing.T) {
	encoded := learnedTable(t)
	if *updateLearnedTable {
		if err := os.WriteFile(learnedTableFile, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", learnedTableFile)
		return
	}
	want, err := os.ReadFile(learnedTableFile)
	if err != nil {
		t.Fatalf("read %s: %v (run with -update-learned-table to record it)", learnedTableFile, err)
	}
	if string(want) != string(encoded) {
		t.Fatalf("learned results differ from %s; first difference:\n%s", learnedTableFile, firstDifference(string(want), string(encoded)))
	}
}

// TestRecordedUsageReordersTheRoot checks the reordering the second table is meant to show, against the first
// table: Calendar and Calculator both start with "cal", so they tie on the match and the usage decides. Chosen for
// "cal", Calendar moves above Calculator, which the golden order lists first. A row chosen at the root is listed
// there, and an app never chosen is not.
func TestRecordedUsageReordersTheRoot(t *testing.T) {
	calendar := indexOfRow(t, learnedUsageFixture(t), "cal", "Calendar")
	calculator := indexOfRow(t, learnedUsageFixture(t), "cal", "Calculator")
	if calendar < 0 || calculator < 0 || calendar >= calculator {
		t.Fatalf("for cal, Calendar (chosen for cal) should outrank Calculator: calendar %d, calculator %d", calendar, calculator)
	}
	if plain := indexOfRow(t, rootTableFixture(t), "cal", "Calendar"); plain < indexOfRow(t, rootTableFixture(t), "cal", "Calculator") {
		t.Fatalf("without usage Calculator should come first: calendar %d", plain)
	}
	if indexOfRow(t, learnedUsageFixture(t), "", "Terminal") < 0 {
		t.Fatal("Terminal was chosen and should be listed at the root")
	}
	if indexOfRow(t, learnedUsageFixture(t), "", "Firefox") >= 0 {
		t.Fatal("Firefox was not chosen in this learner and should not be listed at the root")
	}
}

// indexOfRow returns the position of the row with title in the results of q, or -1.
func indexOfRow(t *testing.T, p *App, q, title string) int {
	t.Helper()
	for i, row := range rootTableRows(t, p, q) {
		if row.Title == title {
			return i
		}
	}
	return -1
}
