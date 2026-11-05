package app

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	clipboardbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/clipboard"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// clipboardBuiltInView opens the Clipboard History built-in's list over the engine's history, as the palette opens it.
func clipboardBuiltInView(t testing.TB, engine *clipEngine) *palette.View {
	t.Helper()
	m := newManagerOverClipboardAt(t, engine, goldenClock())
	return builtInViewPage(t, m, clipboardbuiltin.ID, api.ActionRef{ID: clipboardbuiltin.ListCommandID})
}

// newManagerOverClipboardAt is newManagerOverClipboard with the clock its built-ins read; nil is the real one.
func newManagerOverClipboardAt(t testing.TB, engine *clipEngine, clock platform.Clock) *manager.Manager {
	t.Helper()
	builtins := builtInExtensions(builtInInputs{Zone: time.UTC, Clipboard: defaultClipboardOptions})
	return newManagerWithApps(t, builtins, clock, nil, nil, engine.ports())
}

// defaultClipboardOptions are the settings a new install has.
func defaultClipboardOptions() clipboardbuiltin.Options {
	return clipboardbuiltin.Options{RootList: true, RootSearch: true, LinkPreviews: true}
}

// ports returns the history and pictures of the engine as the extension services read them.
func (e *clipEngine) ports() extensionPorts {
	return extensionPorts{history: func() *clipboard.History { return e.history }, images: e.files}
}
