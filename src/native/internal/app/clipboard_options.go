package app

import clipboardbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/clipboard"

// clipboardOptions is the Clipboard History built-in's settings, read on each call from the settings the app keeps.
func (p *App) clipboardOptions() clipboardbuiltin.Options {
	return clipboardbuiltin.Options{
		RootList:     !p.S.ClipRootOff,
		RootSearch:   !p.S.ClipRootSearchOff,
		LinkPreviews: !p.S.ClipNoLinks,
	}
}
