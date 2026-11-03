package palette

import "github.com/pablopunk/nvm/native/internal/search"

// Confirm returns a view that asks before a destructive run: its first row is the action, its second cancels.
// Cancel and Escape leave the view without running anything. The palette is the only place a confirmation
// happens (CLAUDE.md: no native dialogs).
func Confirm(title, detail string, run func()) *View {
	return &View{
		Title:       "Confirm",
		Placeholder: title + "?",
		Search: func(string) []search.Item {
			return []search.Item{
				{Title: title, Subtitle: detail, Icon: "check", Tone: "red", Actions: []search.Action{
					{Title: "Confirm", Run: func(h search.Host) { h.Hide(); run() }},
				}},
				{Title: "Cancel", Icon: "x", Actions: []search.Action{
					{Title: "Cancel", Dismiss: search.DismissNever, Run: func(h search.Host) { h.Pop() }},
				}},
			}
		},
	}
}
