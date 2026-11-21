package app

import (
	"runtime"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/portable"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// hyperKeyTitle and hyperKeySubtitle are the copy of the Hyper Key setting, as 1.x words it.
const (
	hyperKeyTitle    = "Hyper Key"
	hyperKeySubtitle = "Key combination displayed as Hyper in shortcut labels"
)

// paletteSettingTitle and paletteSettingSubtitle are the copy of the palette shortcut setting, as 1.x words it.
const (
	paletteSettingTitle    = "Open Nevermind Shortcut"
	paletteSettingSubtitle = "Optional global keyboard shortcut to open the palette"
)

// hyperKey is the Hyper key config.json holds. It is read once and kept in step by saveHyperKey. Without a config store
// it is the default and nothing is cached.
func (p *App) hyperKey() string {
	if p.hyper != nil {
		return *p.hyper
	}
	if p.sharedConfig == nil {
		return shortcuts.DefaultHyperKey
	}
	key := shortcuts.DefaultHyperKey
	if doc, err := p.sharedConfig.Load(); err == nil {
		key = shortcuts.HyperKeyOf(doc)
	}
	p.hyper = &key
	return key
}

// hyperRecorderView records the Hyper key. Modifiers alone are a Hyper key, and the recording shows no ✦ of its own.
func (p *App) hyperRecorderView() *palette.View {
	return p.recorderView(&shortcutRecorder{
		entry:  shortcuts.Entry{Title: hyperKeyTitle},
		labels: shortcuts.Labeler{GOOS: runtime.GOOS},
		hyper:  true,
	})
}

// saveHyperKey writes the recorded Hyper key to settings.hyperKey in config.json, where 1.x reads it.
func (p *App) saveHyperKey(h search.Host, accelerator string) {
	if p.sharedConfig == nil {
		h.Toast(errNoSharedConfig.Error())
		return
	}
	hyper := shortcuts.Normalize(accelerator)
	if err := p.sharedConfig.Update(func(doc portable.Document) {
		doc.SetValue(hyper, "settings", "hyperKey")
	}); err != nil {
		h.Toast(err.Error())
		return
	}
	p.hyper = &hyper
	h.Pop()
	h.Toast("Hyper key saved")
}
