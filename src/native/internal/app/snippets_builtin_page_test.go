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
func builtInViewPage(t testing.TB, m *manager.Manager, extID string, ref api.ActionRef) *palette.View {
	t.Helper()
	return builtInInvocationPage(t, m, extID, api.Invocation{Ref: ref})
}

// builtInInvocationPage runs an invocation of a built-in and returns the page its view opens. A conflict needs the
// draft the editor kept, so an invocation can carry more than the action.
func builtInInvocationPage(t testing.TB, m *manager.Manager, extID string, in api.Invocation) *palette.View {
	t.Helper()
	return extview.NewSession(syncViewDeps(m), extID, builtInInvocationView(t, m, extID, in)).Palette()
}

// builtInInvocationView runs an invocation of a built-in and returns the view it opens.
func builtInInvocationView(t testing.TB, m *manager.Manager, extID string, in api.Invocation) api.View {
	t.Helper()
	result, failure, err := m.Run(context.Background(), extID, in)
	if err != nil || failure != nil || result.View == nil {
		t.Fatalf("run %s: err %v failure %+v, view %v", in.Ref.ID, err, failure, result.View != nil)
	}
	return *result.View
}
