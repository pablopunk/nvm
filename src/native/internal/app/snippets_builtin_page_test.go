package app

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/snippets"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
)

// snippetsBuiltInPage runs the Snippets built-in's ref over the legacy snippets and returns the palette page the view
// opens, through the same extension view path the palette takes for every built-in view.
func snippetsBuiltInPage(t *testing.T, legacy []snippets.Snippet, ref string, args map[string]string) *palette.View {
	t.Helper()
	m := newManagerOver(t, builtInExtensions(builtInInputs{Zone: time.UTC, Snippets: legacy}))
	return builtInViewPage(t, m, snippets.ID, api.ActionRef{ID: ref, Args: args})
}

// builtInViewPage runs a built-in's action and returns the page its view opens.
func builtInViewPage(t *testing.T, m *manager.Manager, extID string, ref api.ActionRef) *palette.View {
	t.Helper()
	result, failure, err := m.Run(context.Background(), extID, api.Invocation{Ref: ref})
	if err != nil || failure != nil || result.View == nil {
		t.Fatalf("run %s: err %v failure %+v, view %v", ref.ID, err, failure, result.View != nil)
	}
	return extview.NewSession(syncViewDeps(m), extID, *result.View).Palette()
}
