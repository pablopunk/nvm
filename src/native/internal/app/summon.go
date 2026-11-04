package app

import (
	"errors"
	"fmt"
	"log"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// notifySummon delivers a status change of the palette shortcut. Retries arrive on a background goroutine, so the
// banner is drawn on the window's thread; before the window exists the change is shown directly.
func (p *App) notifySummon(st shortcuts.Status) {
	if p.panel == nil {
		p.summonChanged(st)
		return
	}
	p.panel.Update(func() { p.summonChanged(st) })
}

// summonChanged shows the status banner and, the first time the shortcut is known to be unavailable, the palette.
func (p *App) summonChanged(st shortcuts.Status) {
	p.showSummonNotice(st)
	if !st.Unavailable() {
		return
	}
	log.Println("shortcut", st.Accelerator, "is unavailable")
	if !p.launchShown {
		p.launchShown = true
		p.Show()
	}
}

// showSummonNotice shows the banner for st, or clears it when the shortcut works. Its action opens the palette
// shortcut recorder, the one place the shortcut is changed.
func (p *App) showSummonNotice(st shortcuts.Status) {
	copy, show := shortcuts.SummonNotice(st, shortcutLabelOf)
	if !show {
		p.SetNotice(nil)
		return
	}
	p.SetNotice(&palette.Notice{
		Icon:        "keyboard",
		Title:       copy.Title,
		Detail:      copy.Detail,
		ActionLabel: copy.ActionLabel,
		Alert:       copy.Alert,
		Action:      func() { p.Push(p.paletteRecorderView()) },
	})
}

// changeHotkeyMessage says why the OS refused a new palette shortcut. The previous one stays bound.
func changeHotkeyMessage(attempt, kept string) string {
	return fmt.Sprintf("“%s” is taken by another app or is not a shortcut. %s stays.", attempt, kept)
}

// paletteChangeFailureMessage says why a new palette shortcut was not saved. The document names the action that already
// holds the key; only a refusal from the OS blames another app.
func paletteChangeFailureMessage(err error, attempt, kept string) string {
	if errors.Is(err, platform.ErrConflict) {
		return changeHotkeyMessage(attempt, kept)
	}
	return err.Error()
}
