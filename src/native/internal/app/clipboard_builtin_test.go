package app

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	clipboardbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/clipboard"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// clipboardBuiltInView opens the Clipboard History built-in's list over the engine's history, as the palette opens it.
func clipboardBuiltInView(t testing.TB, engine *clipEngine) *palette.View {
	t.Helper()
	m := newManagerOverClipboard(t, engine)
	return builtInViewPage(t, m, clipboardbuiltin.ID, api.ActionRef{ID: clipboardbuiltin.ListCommandID})
}

// newManagerOverClipboard is a manager with the built-ins, whose clipboard history and pictures are the engine's. The
// clipboard port stays empty: the list and the removals read the history only.
func newManagerOverClipboard(t testing.TB, engine *clipEngine) *manager.Manager {
	t.Helper()
	builtins := builtInExtensions(builtInInputs{Zone: time.UTC, Clipboard: defaultClipboardOptions})
	return newManagerWithApps(t, builtins, nil, nil, nil, engine.ports())
}

// defaultClipboardOptions are the settings a new install has.
func defaultClipboardOptions() clipboardbuiltin.Options {
	return clipboardbuiltin.Options{RootList: true, RootSearch: true, LinkPreviews: true}
}
