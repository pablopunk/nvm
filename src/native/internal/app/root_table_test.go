package app

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/quicklinks"
	"github.com/pablopunk/nvm/native/internal/search"
)

// updateRootTable rewrites testdata/queries-expected.json from the current root search. The file was
// generated once from the prototype's rootSearch, before the search coordinator existed; later stages
// must reproduce it, so it is regenerated only by a deliberate change of the root ranking.
var updateRootTable = flag.Bool("update-root-table", false, "rewrite testdata/queries-expected.json")

// rootTableQueriesFile is the canned query list of the parity review, one query per line. The first line is the empty
// query. The parity sheet (testdata/parity/parity-review.md) and cutover.md cite the same file.
const rootTableQueriesFile = "../../testdata/queries.txt"

// rootTableQueryCount is the number of canned queries the parity review compares.
const rootTableQueryCount = 20

// loadRootTableQueries reads the canned queries, in file order.
func loadRootTableQueries(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(rootTableQueriesFile)
	if err != nil {
		t.Fatalf("read %s: %v", rootTableQueriesFile, err)
	}
	queries := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(queries) != rootTableQueryCount {
		t.Fatalf("%s has %d queries, want %d", rootTableQueriesFile, len(queries), rootTableQueryCount)
	}
	return queries
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
func rootTableFixture(t *testing.T) *App {
	t.Helper()
	p := goldenPalette(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	hackerNews := []quicklinks.Link{{Name: "Hacker News", Keyword: "hn", URL: "https://hn.algolia.com/?q={query}"}}
	p.exts.mgr.Store(newManagerWithApps(t, builtInExtensions(builtInInputs{Rates: p.rateCache, Zone: p.zone, Quicklinks: hackerNews}), nil, nil, p.appsvc))
	p.files = newTestFiles(t, home, []string{
		filepath.Join(home, "Documents", "meeting-notes.md"),
		filepath.Join(home, "Documents", "release-plan.txt"),
		filepath.Join(home, "Desktop", "Settings.png"),
		filepath.Join(home, "Downloads", "clipboard-export.csv"),
		filepath.Join(home, "Pictures", "cloud-setup.jpg"),
	})
	return p
}

// rootResultsForTable is the root search the table exercises: a fresh root view, searched synchronously the way
// the palette searches it.
func rootResultsForTable(p *App, q string) []search.Item {
	p.SetStack(p.rootView())
	p.SetQuery(q)
	return p.Results()
}

// homeRelativeSlashes writes text with forward slashes and the home folder replaced by $HOME.
func homeRelativeSlashes(text, home string) string {
	return strings.ReplaceAll(filepath.ToSlash(text), filepath.ToSlash(home), "$HOME")
}

// rootTableRows runs the root search for q and reduces each row to its comparable part. Home-relative
// paths are rewritten to $HOME with forward slashes, since the fixture home is a temporary directory and
// Windows joins paths with backslashes.
func rootTableRows(t *testing.T, p *App, q string) []rootRow {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	items := rootResultsForTable(p, q)
	rows := make([]rootRow, 0, len(items))
	for _, it := range items {
		row := rootRow{
			Kind: it.Kind, Title: it.Title, Subtitle: homeRelativeSlashes(it.Subtitle, home), Section: it.Section,
			ID: homeRelativeSlashes(it.ID, home), Icon: it.Icon, Tone: it.Tone, Accessory: it.Accessory, Prominent: it.Prominent, TintTitle: it.TintTitle,
			IconPath: homeRelativeSlashes(it.IconPath, home),
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
	queries := loadRootTableQueries(t)
	got := map[string][]rootRow{}
	for _, q := range queries {
		got[q] = rootTableRows(t, rootTableFixture(t), q)
	}
	encoded, err := json.MarshalIndent(orderedTable(got, queries), "", "  ")
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

func orderedTable(got map[string][]rootRow, queries []string) []tableEntry {
	out := make([]tableEntry, 0, len(queries))
	for _, q := range queries {
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
