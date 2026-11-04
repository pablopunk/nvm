package app

import (
	"context"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/search"
)

// collectExtensionRows runs the root's extension search for q on a real manager and returns every row it emits.
func collectExtensionRows(t *testing.T, p *App, q string) []search.Item {
	t.Helper()
	var rows []search.Item
	extensionProvider(p).Search(context.Background(), q, func(batch search.Batch) {
		rows = append(rows, batch.Items...)
	})
	return rows
}

func TestFailedExtensionYieldsAFailedRow(t *testing.T) {
	m := newFixtureManager(t, "acme.syntax")
	waitForState(t, m, "acme.syntax", supervisor.StateBroken)
	p := &App{}
	p.exts.mgr.Store(m)

	row := findRowTitled(t, collectExtensionRows(t, p, "alpha"), "Broken source fixture is not responding")
	if row.Accessory != "Failed" || !strings.Contains(row.Subtitle, "does not load") {
		t.Fatalf("failed row = accessory %q subtitle %q, want Failed with the reason", row.Accessory, row.Subtitle)
	}
}

func TestRunningExtensionYieldsItsItemRows(t *testing.T) {
	m := newFixtureManager(t, "acme.good")
	waitForState(t, m, "acme.good", supervisor.StateRunning)
	p := &App{}
	p.exts.mgr.Store(m)

	rows := collectExtensionRows(t, p, "alpha")
	item := findRowTitled(t, rows, "alpha")
	if item.Kind != "Good fixture" || item.Accessory != "" {
		t.Fatalf("item row = kind %q accessory %q, want the extension's item", item.Kind, item.Accessory)
	}
	for _, row := range rows {
		if row.Accessory == "Failed" {
			t.Fatalf("running extension yielded a failed row %q", row.Title)
		}
	}
}
