package app

import (
	"fmt"
	"log"
	"runtime"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// startSummon binds the palette's shortcut. When it stays unbound, the palette opens once at launch, as the
// Electron app does, so the user is never left without a way in.
func (p *App) startSummon() {
	p.summon = shortcuts.NewSummon(shortcuts.SummonOptions{
		Hotkeys:  p.ports.Hotkeys,
		Clock:    p.ports.Clock,
		GOOS:     runtime.GOOS,
		Toggle:   p.Toggle,
		OnChange: func(st shortcuts.Status) { p.panel.Update(func() { p.summonChanged(st) }) },
	})
	p.summonChanged(p.summon.Start(p.S.Hotkey))
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

// showSummonNotice shows the banner for st, or clears it when the shortcut works.
func (p *App) showSummonNotice(st shortcuts.Status) {
	copy, show := shortcuts.SummonNotice(st, func(accelerator string) string { return accelerator })
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
		Action:      func() { p.Push(p.hotkeyView()) },
	})
}

// changeHotkeyMessage says why a new shortcut was not saved. Summon.Change binds the previous one again first.
func changeHotkeyMessage(attempt, kept string) string {
	return fmt.Sprintf("“%s” is taken by another app or is not a shortcut. %s stays.", attempt, kept)
}
