package app

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

func TestTypingACommandTitleFindsTheCommandOfARunningExtension(t *testing.T) {
	m := newFixtureManager(t, "acme.views")
	waitForState(t, m, "acme.views", supervisor.StateRunning)
	p := &App{}
	p.exts.mgr.Store(m)

	findRowTitled(t, collectExtensionRows(t, p, "list fixture"), "List fixture")
	if rows := collectExtensionRows(t, p, "zz no such command"); len(rows) != 0 {
		t.Fatalf("query naming no command showed %d rows", len(rows))
	}
}

func TestRootStillListsTheCommandsOfARunningExtension(t *testing.T) {
	m := newFixtureManager(t, "acme.views")
	waitForState(t, m, "acme.views", supervisor.StateRunning)
	p := &App{}
	p.exts.mgr.Store(m)

	findRowTitled(t, extensionProvider(p).Root(context.Background()), "List fixture")
}

func TestRootShowsTheRootItemsOfARunningExtension(t *testing.T) {
	m := newFixtureManager(t, "acme.good")
	waitForState(t, m, "acme.good", supervisor.StateRunning)
	p := &App{}
	p.exts.mgr.Store(m)

	findRowTitled(t, extensionProvider(p).Root(context.Background()), "Good root")
}
