package palette

import "github.com/pablopunk/nvm/native/internal/search"

// Confirm returns a view that asks before a destructive run: its first row is the action, its second cancels.
// Cancel and Escape leave the view without running anything. The palette is the only place a confirmation
// happens (CLAUDE.md: no native dialogs). Confirm adds the question mark to the title, so callers pass the bare
// title ("Log Out", not "Log Out?").
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

// Decide is Confirm that reports the answer exactly once. answer(true) runs when the action row runs; answer(false)
// runs when Cancel runs, when Escape is pressed, when the view is left, or when the palette hides with the view open.
// The view stays on the stack after a hide, and the next Show leaves it without a second answer.
func Decide(title, detail string, answer func(confirmed bool)) *View {
	answered := false
	resolve := func(confirmed bool) {
		if !answered {
			answered = true
			answer(confirmed)
		}
	}
	return &View{
		Title:       "Confirm",
		Placeholder: title + "?",
		OnEscape:    func() bool { resolve(false); return false },
		OnLeave:     func() { resolve(false) },
		OnHide:      func() { resolve(false) },
		Search: func(string) []search.Item {
			return []search.Item{
				{Title: title, Subtitle: detail, Icon: "check", Tone: "red", Actions: []search.Action{
					{Title: "Confirm", Run: func(h search.Host) { resolve(true); h.Hide() }},
				}},
				{Title: "Cancel", Icon: "x", Actions: []search.Action{
					{Title: "Cancel", Dismiss: search.DismissNever, Run: func(h search.Host) { resolve(false); h.Pop() }},
				}},
			}
		},
	}
}
