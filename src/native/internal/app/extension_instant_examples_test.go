package app

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestGitHubExamplesReturnCachedLoaderViewsWithoutRunningACommand(t *testing.T) {
	for _, fixture := range []struct{ id, command string }{
		{"acme.githubrepos", "show-repos"},
		{"acme.githubissues", "show-my-issues"},
	} {
		t.Run(fixture.id, func(t *testing.T) {
			m := newFixtureManager(t, fixture.id)
			waitForState(t, m, fixture.id, supervisor.StateRunning)
			result, failure, err := m.Run(context.Background(), fixture.id, api.Invocation{Ref: api.ActionRef{ID: fixture.command}})
			if err != nil || failure != nil || result.View == nil || result.Next != nil {
				t.Fatalf("opening returned an error or token prompt: result=%+v failure=%+v err=%v", result, failure, err)
			}
			if source := result.View.Source; source == nil || source.Loader == "" || source.CacheKey == "" {
				t.Fatalf("opening did not return a cached loader shell: %+v", result.View)
			}
		})
	}
}

func TestWarmRootFirstBatchRetainsEveryCachedExtensionRow(t *testing.T) {
	p := goldenPalette(t)
	m := newFixtureManager(t, "acme.good")
	waitForState(t, m, "acme.good", supervisor.StateRunning)
	p.exts.mgr.Store(m)
	p.rootRows.rows = map[string]manager.SearchResult{
		"acme.good": {ExtensionID: "acme.good", Title: "Good fixture", Items: []api.Item{{ID: "cached", Title: "Cached root"}}},
	}
	var first []search.Item
	seen := false
	extensionProvider(p, &rootInput{}).(search.RootStreamer).RootStream(context.Background(), func(batch search.Batch) {
		if !seen {
			first, seen = batch.Items, true
		}
	})
	findRowTitled(t, first, "Cached root")
}

func TestUnusedExtensionCommandsStartBelowAppsAndCarryStableLearningKeys(t *testing.T) {
	p := goldenPalette(t)
	m := p.extensionManager()
	result := manager.CommandResult{ExtensionID: "acme.example", Title: "Example", Commands: []api.Command{{ID: "run", Title: "Run Example"}}}
	root := matchingCommandRows(p, m, result, "", nil, nil)
	typed := matchingCommandRows(p, m, result, "run example", nil, nil)
	if len(root) != 1 || root[0].Score >= scoreRootApplication || root[0].LearnKey == "" || root[0].ID == "" {
		t.Fatalf("root has no useful default app preference/learning identity: %+v", root)
	}
	if len(typed) != 1 || typed[0].Score != scoreExtension || typed[0].ID != root[0].ID {
		t.Fatalf("typed discovery changed with root defaults: %+v", typed)
	}
}
