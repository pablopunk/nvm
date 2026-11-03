package app

import (
	"encoding/json"
	"flag"
	"github.com/pablopunk/nvm/native/internal/search"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// updateRootTable rewrites testdata/queries-expected.json from the current root search. The file was
// generated once from the prototype's rootSearch, before the search coordinator existed; later stages
// must reproduce it, so it is regenerated only by a deliberate change of the root ranking.
var updateRootTable = flag.Bool("update-root-table", false, "rewrite testdata/queries-expected.json")

// rootTableQueries are the twenty canned queries of the parity review (PLAN.md section 3.3): empty,
// application and command matches, calculator and units, timers, quicklinks, snippets, extensions,
// files, clipboard text, conversations, URLs and the automate fallback.
var rootTableQueries = []string{
	"",
	"cl",
	"clip",
	"clipboard",
	"calc",
	"2+2",
	"12 km to mi",
	"timer 5m",
	"hn rust",
	";sig",
	"fixture",
	"set",
	"term",
	"fire",
	"meeting",
	"release",
	"time",
	"github.com",
	"open github.com",
	"show the current time in tokyo and london",
}

// rootRow is the part of a result row the parity table compares: what the row says and what its actions
// are called. Run closures cannot be compared, so action titles and labels stand in for them.
type rootRow struct {
	Kind      string       `json:"kind"`
	Title     string       `json:"title"`
	Subtitle  string       `json:"subtitle"`
	Section   string       `json:"section"`
	ID        string       `json:"id"`
	Icon      string       `json:"icon"`
	Tone      string       `json:"tone"`
	Accessory string       `json:"accessory"`
	Prominent bool         `json:"prominent"`
	TintTitle bool         `json:"tintTitle"`
	IconPath  string       `json:"iconPath"`
	Actions   []rootAction `json:"actions"`
}

type rootAction struct {
	Title string `json:"title"`
	Label string `json:"label"`
}

const rootTableFile = "testdata/queries-expected.json"

// rootTableFixture is the palette the table runs against: the golden fixtures plus one quicklink, one
// extension, and an indexed file tree under the fixture home, so every provider has something to say.
func rootTableFixture(t *testing.T) *Palette {
	t.Helper()
	p := goldenPalette(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	p.S.Quicklinks = []Quicklink{{Name: "Hacker News", Keyword: "hn", URL: "https://hn.algolia.com/?q={query}"}}
	p.exts = []loadedExt{{Extension: Extension{ID: "user.fixture", Title: "Fixture", Search: fixtureExtensionSearch}, File: "fixture.go"}}
	p.files = &FileIndex{done: true, paths: []string{
		filepath.Join(home, "Documents", "meeting-notes.md"),
		filepath.Join(home, "Documents", "release-plan.txt"),
		filepath.Join(home, "Desktop", "Settings.png"),
		filepath.Join(home, "Downloads", "clipboard-export.csv"),
		filepath.Join(home, "Pictures", "cloud-setup.jpg"),
	}}
	return p
}

// fixtureExtensionSearch answers queries that mention the word "fix" with two rows, and nothing else.
func fixtureExtensionSearch(q string) []ExtItem {
	if !strings.Contains(strings.ToLower(q), "fix") {
		return nil
	}
	return []ExtItem{
		{Title: "Fix result for " + q, Subtitle: "fixture", Value: "fixed"},
		{Title: "Fixture docs", Subtitle: "fixture", URL: "https://example.com/fixture"},
	}
}

// rootResultsForTable is the root search the table exercises: a fresh root view, searched synchronously the way
// the palette searches it.
func rootResultsForTable(p *Palette, q string) []search.Item {
	p.stack = []*View{p.rootView()}
	p.query = q
	p.search()
	return p.results
}

// rootTableRows runs the root search for q and reduces each row to its comparable part. Home-relative
// paths are rewritten to $HOME, since the fixture home is a temporary directory.
func rootTableRows(t *testing.T, p *Palette, q string) []rootRow {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	items := rootResultsForTable(p, q)
	rows := make([]rootRow, 0, len(items))
	for _, it := range items {
		row := rootRow{
			Kind: it.Kind, Title: it.Title, Subtitle: strings.ReplaceAll(it.Subtitle, home, "$HOME"), Section: it.Section,
			ID: strings.ReplaceAll(it.ID, home, "$HOME"), Icon: it.Icon, Tone: it.Tone, Accessory: it.Accessory, Prominent: it.Prominent, TintTitle: it.TintTitle,
			IconPath: strings.ReplaceAll(it.IconPath, home, "$HOME"),
		}
		for _, a := range it.Actions {
			row.Actions = append(row.Actions, rootAction{Title: a.Title, Label: a.Label})
		}
		rows = append(rows, row)
	}
	return rows
}

// TestRootResultsMatchRecordedTable compares the root search for the twenty canned queries with the table
// recorded from the prototype. Any reordering, dropped row or changed section fails it.
func TestRootResultsMatchRecordedTable(t *testing.T) {
	got := map[string][]rootRow{}
	for _, q := range rootTableQueries {
		got[q] = rootTableRows(t, rootTableFixture(t), q)
	}
	encoded, err := json.MarshalIndent(orderedTable(got), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	if *updateRootTable {
		if err := os.WriteFile(rootTableFile, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", rootTableFile)
		return
	}
	want, err := os.ReadFile(rootTableFile)
	if err != nil {
		t.Fatalf("read %s: %v (run with -update-root-table to record it)", rootTableFile, err)
	}
	if string(want) != string(encoded) {
		t.Fatalf("root results differ from %s; first difference:\n%s", rootTableFile, firstDifference(string(want), string(encoded)))
	}
}

// tableEntry keeps the queries in their canned order in the JSON file.
type tableEntry struct {
	Query string    `json:"query"`
	Rows  []rootRow `json:"rows"`
}

func orderedTable(got map[string][]rootRow) []tableEntry {
	out := make([]tableEntry, 0, len(rootTableQueries))
	for _, q := range rootTableQueries {
		rows := got[q]
		if rows == nil {
			rows = []rootRow{}
		}
		out = append(out, tableEntry{Query: q, Rows: rows})
	}
	return out
}

// firstDifference shows the first line where two renderings of the table disagree.
func firstDifference(want, got string) string {
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			return "line " + itoa(i+1) + "\n  recorded: " + w + "\n  current:  " + g
		}
	}
	return "no line differs"
}
