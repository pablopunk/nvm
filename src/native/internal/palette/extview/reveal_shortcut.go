package extview

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// revealShortcut is the standard key of a palette result that is a file: it shows the file in the file manager.
const revealShortcut = "CommandOrControl+Enter"

// WithRevealShortcut gives a row with a file the standard Command+Enter reveal. A row whose own action already binds
// that key keeps it. A row with a reveal action that has no key takes the key for it. Any other row with a File gets a
// reveal action added, so every file result answers the key the same way. The row's slices are copied, never changed,
// because the items are shared with the view's snapshot.
func WithRevealShortcut(item api.Item, title string) api.Item {
	if item.Disabled || bindsRevealKey(item) {
		return item
	}
	assigned := false
	assign := func(action *api.Action) {
		if !assigned && action.Kind == api.ActionRevealPath && action.Path != "" && action.Shortcut == "" {
			action.Shortcut, assigned = revealShortcut, true
		}
	}
	if item.Primary != nil {
		primary := *item.Primary
		assign(&primary)
		item.Primary = &primary
	}
	item.Actions = append([]api.Action(nil), item.Actions...)
	for i := range item.Actions {
		assign(&item.Actions[i])
	}
	if item.ActionPanel != nil {
		panel := *item.ActionPanel
		panel.Sections = append([]api.ActionSection(nil), item.ActionPanel.Sections...)
		for i := range panel.Sections {
			panel.Sections[i].Actions = append([]api.Action(nil), panel.Sections[i].Actions...)
			for j := range panel.Sections[i].Actions {
				assign(&panel.Sections[i].Actions[j])
			}
		}
		item.ActionPanel = &panel
	}
	if assigned || item.File == nil || item.File.Path == "" {
		return item
	}
	reveal := api.RevealPath(orDefault(title, "Reveal in File Manager"), item.File.Path)
	reveal.Shortcut = revealShortcut
	item.Actions = append(item.Actions, reveal)
	return item
}

// bindsRevealKey reports whether any action of the row already uses Command+Enter.
func bindsRevealKey(item api.Item) bool {
	binds := func(action api.Action) bool {
		mods, key, ok := parseShortcut(action.Shortcut)
		return ok && mods == palette.Mod && key == ui.KeyEnter
	}
	if item.Primary != nil && binds(*item.Primary) {
		return true
	}
	for _, action := range item.Actions {
		if binds(action) {
			return true
		}
	}
	if item.ActionPanel != nil {
		for _, section := range item.ActionPanel.Sections {
			for _, action := range section.Actions {
				if binds(action) {
					return true
				}
			}
		}
	}
	return false
}
