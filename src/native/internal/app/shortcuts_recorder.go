package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// shortcutRecorder holds the shortcut the last key press made for entry, until Enter saves it.
type shortcutRecorder struct {
	entry       shortcuts.Entry
	accelerator string
	focus       ui.Handle
}

// shortcutRecorderView records the next key press as the shortcut of entry. Enter saves it and Escape goes back. The
// bindings are released while it is open, so its keys reach it rather than running the actions they name.
func (p *App) shortcutRecorderView(entry shortcuts.Entry) *palette.View {
	manager := p.shortcutsManager()
	recorder := &shortcutRecorder{entry: entry}
	manager.Suspend()
	return &palette.View{
		Title:     "Record Shortcut",
		HideInput: true,
		Hint:      "↵ Save   esc Back",
		Body:      func(c *ui.Context, h search.Host) { recorder.draw(c) },
		OnKey:     func(c *ui.Context, h search.Host) bool { return p.recorderKeys(c, h, recorder) },
		OnLeave: func() {
			if manager.Suspended() {
				manager.Resume()
			}
		},
	}
}

// recorderKeys takes the keys the input does not: Escape goes back and Enter saves what was recorded.
func (p *App) recorderKeys(c *ui.Context, h search.Host, recorder *shortcutRecorder) bool {
	switch {
	case c.Shortcut(0, ui.KeyEscape):
		h.Pop()
	case c.Shortcut(0, ui.KeyEnter):
		p.saveRecorded(h, recorder)
	default:
		return false
	}
	return true
}

func (p *App) saveRecorded(h search.Host, recorder *shortcutRecorder) {
	if recorder.accelerator == "" {
		h.Toast("Press a keyboard shortcut first")
		return
	}
	manager := p.shortcutsManager()
	if recorder.entry.ActionID == shortcuts.PaletteID {
		change, err := manager.SetPalette(recorder.accelerator)
		if err != nil {
			h.Toast(paletteChangeFailureMessage(err, shortcutLabelOf(recorder.accelerator), shortcutLabelOf(manager.PaletteAccelerator())))
			return
		}
		p.refreshMenubarMenu()
		h.Pop()
		h.Toast(paletteChangeMessage(change))
		return
	}
	if err := manager.SetAction(recorder.entry.ActionID, recorder.accelerator); err != nil {
		h.Toast(err.Error())
		return
	}
	h.Pop()
	h.Toast("Shortcut set: " + shortcutLabelOf(recorder.accelerator))
}

// draw shows the recorded shortcut as keycaps, or asks for one. The element takes the keys while it is focused. The
// palette moves focus to its search field when a view is pushed, so the element asks for focus on every frame it
// does not have it.
func (r *shortcutRecorder) draw(c *ui.Context) {
	k := design.Tokens(c)
	ui.Column(c).Bind(&r.focus).Focusable().HandleInput(r.handleInput).Padding(24).Gap(14).Center().Children(func() {
		if r.accelerator == "" {
			if held := shortcuts.ModifierNames(recordedModifiers(c.Modifiers())); len(held) > 0 {
				ui.Row(c).Gap(6).Center().Children(func() {
					for _, name := range held {
						design.Keycap(c, shortcutLabelOf(name))
					}
				})
				ui.Text(c, "Press a key with them").FontSize(11).TextColor(k.Muted)
				return
			}
			ui.Text(c, "Press a keyboard shortcut").FontSize(13).FontWeight(600).TextColor(k.Strong)
			ui.Text(c, "Use at least one modifier, then press Enter").FontSize(11).TextColor(k.Muted)
			return
		}
		ui.Row(c).Gap(6).Center().Children(func() {
			for _, part := range shortcuts.Parts(r.accelerator) {
				design.Keycap(c, shortcutLabelOf(part))
			}
		})
		ui.Text(c, "Save shortcut for “"+r.entry.Title+"”").FontSize(11).TextColor(k.Muted)
	})
	if !c.Focused(r.focus) {
		r.focus.Focus(c)
	}
}

// handleInput records the key a press makes, with the modifiers held. Enter and Escape go on to the palette.
func (r *shortcutRecorder) handleInput(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputKeyDown {
		return false
	}
	key := ev.Key
	if key == ui.KeyUnknown {
		key = ev.PhysicalKey
	}
	if key == ui.KeyEnter || key == ui.KeyEscape {
		return false
	}
	if accelerator := shortcuts.Capture(recordedModifiers(ev.Mods), palette.KeyName(key)); accelerator != "" {
		r.accelerator = accelerator
	}
	return true
}

// recordedModifiers names the held modifiers the way accelerators do: Super is Command on every platform.
func recordedModifiers(mods ui.Modifiers) shortcuts.Modifiers {
	return shortcuts.Modifiers{
		Command: mods&ui.Super != 0,
		Control: mods&ui.Ctrl != 0,
		Alt:     mods&ui.Alt != 0,
		Shift:   mods&ui.Shift != 0,
	}
}
