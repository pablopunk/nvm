package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// stage draws what the recorder shows: the recorded keys, the modifiers held so far, or the prompt for a shortcut.
func (r *shortcutRecorder) stage(c *ui.Context) {
	if r.accelerator != "" {
		keyRow(c, r.labels.Keycaps(r.accelerator))
		stageCaption(c, "Save shortcut for “"+r.entry.Title+"”")
		return
	}
	if held := shortcuts.ModifierNames(recordedModifiers(c.Modifiers())); len(held) > 0 {
		keyRow(c, heldKeyLabels(r.labels, held))
		stageCaption(c, "Press a key with them")
		return
	}
	shortcutPrompt(c)
}

// keyRow lays the large keys of a stage out in one centred row.
func keyRow(c *ui.Context, labels []string) {
	ui.Row(c).Gap(14).Center().Children(func() {
		for _, label := range labels {
			design.KeycapLarge(c, label)
		}
	})
}

// heldKeyLabels spells each held modifier as its key label, in the order the names came.
func heldKeyLabels(labeler shortcuts.Labeler, names []string) []string {
	labels := make([]string, len(names))
	for i, name := range names {
		labels[i] = labeler.Label(name)
	}
	return labels
}

// stageCaption is the line under the keys of a stage.
func stageCaption(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(13).TextColor(design.Tokens(c).Muted)
}

// shortcutPrompt asks for a shortcut while none is held or recorded.
func shortcutPrompt(c *ui.Context) {
	k := design.Tokens(c)
	ui.Column(c).Gap(4).Center().Children(func() {
		ui.Text(c, "Press a keyboard shortcut").FontSize(15).FontWeight(600).TextColor(k.Strong)
		ui.Text(c, "Use at least one modifier, then press Enter").FontSize(12).TextColor(k.Muted)
	})
}
