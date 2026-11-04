package palette

import "github.com/pablopunk/nvm/native/internal/search"

// Confirm returns a view that asks before a destructive run: its first row is the action, its second cancels.
// Cancel and Escape leave the view without running anything. The palette is the only place a confirmation
// happens (CLAUDE.md: no native dialogs). Confirm adds the question mark to the title, so callers pass the bare
// title ("Log Out", not "Log Out?").
func Confirm(title, detail string, run func()) *View {
	return Decide(title, detail, func(confirmed bool) {
		if confirmed {
			run()
		}
	})
}

// Decide is Confirm that reports the answer: answer(true) when the action row runs, answer(false) when Cancel runs.
// Escape leaves the view without an answer, so a caller that waits for one must bound its wait.
func Decide(title, detail string, answer func(confirmed bool)) *View {
	return &View{
		Title:       "Confirm",
		Placeholder: title + "?",
		Search: func(string) []search.Item {
			return []search.Item{
				{Title: title, Subtitle: detail, Icon: "check", Tone: "red", Actions: []search.Action{
					{Title: "Confirm", Run: func(h search.Host) { h.Hide(); answer(true) }},
				}},
				{Title: "Cancel", Icon: "x", Actions: []search.Action{
					{Title: "Cancel", Dismiss: search.DismissNever, Run: func(h search.Host) { h.Pop(); answer(false) }},
				}},
			}
		},
	}
}
