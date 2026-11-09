package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/ui"
)

// revealShortcut is the standard key of a palette result that is a file: it shows the file in the file manager.
const revealShortcut = "CommandOrControl+Enter"

// withRevealShortcut gives a row with a file the standard Command+Enter reveal. A row whose own action already binds
// that key keeps it. A row with a reveal action that has no key takes the key for it. Any other row with a File gets a
// reveal action added, so every file result answers the key the same way. The row's slices are copied, never changed,
// because the items are shared with the view's snapshot.
func withRevealShortcut(item api.Item, title string) api.Item {
	if item.Disabled {
		return item
	}
	taken, free := false, false
	item.Primary = markReveal(item.Primary, &taken, &free)
	item.Actions = markRevealAll(item.Actions, &taken, &free)
	if item.ActionPanel != nil {
		panel := *item.ActionPanel
		panel.Sections = make([]api.ActionSection, len(item.ActionPanel.Sections))
		for i, section := range item.ActionPanel.Sections {
			section.Actions = markRevealAll(section.Actions, &taken, &free)
			panel.Sections[i] = section
		}
		item.ActionPanel = &panel
	}
	if taken || free || item.File == nil || item.File.Path == "" {
		return item
	}
	reveal := api.RevealPath(orDefault(title, "Reveal in File Manager"), item.File.Path)
	reveal.Shortcut = revealShortcut
	item.Actions = append(append([]api.Action(nil), item.Actions...), reveal)
	return item
}

func markReveal(action *api.Action, taken, free *bool) *api.Action {
	if action == nil {
		return nil
	}
	copied := markRevealAll([]api.Action{*action}, taken, free)
	return &copied[0]
}

// markRevealAll walks actions once, noting whether one binds the reveal key, and gives the first unbound reveal action
// the key when none does.
func markRevealAll(actions []api.Action, taken, free *bool) []api.Action {
	out := append([]api.Action(nil), actions...)
	for i := range out {
		if mods, key, ok := parseShortcut(out[i].Shortcut); ok && mods == palette.Mod && key == ui.KeyEnter {
			*taken = true
		}
	}
	if *taken {
		return out
	}
	for i := range out {
		if out[i].Kind == api.ActionRevealPath && out[i].Path != "" && out[i].Shortcut == "" && !*free {
			out[i].Shortcut = revealShortcut
			*free = true
		}
	}
	return out
}
