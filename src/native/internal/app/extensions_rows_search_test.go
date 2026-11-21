package app

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

func TestTypingACommandTitleFindsTheCommandOfARunningExtension(t *testing.T) {
	p, m, _ := awaitFixtureCommand(t, "acme.views", "list fixture", "List fixture")
	manifest, _, err := m.Definition("acme.views")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range collectExtensionRows(t, p, "zz no such command") {
		if row.Kind == manifest.Title {
			t.Fatalf("query naming no command showed %q from the fixture", row.Title)
		}
	}
}

func TestRootStillListsTheCommandsOfARunningExtension(t *testing.T) {
	awaitFixtureCommand(t, "acme.views", "", "List fixture")
}

func TestRootShowsTheRootItemsOfARunningExtension(t *testing.T) {
	m := newFixtureManager(t, "acme.good")
	waitForState(t, m, "acme.good", supervisor.StateRunning)
	p := &App{}
	p.exts.mgr.Store(m)

	findRowTitled(t, extensionProvider(p, &rootInput{}).Root(context.Background()), "Good root")
}
