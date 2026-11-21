package shortcuts

import "strings"

// Canonical storage spells an accelerator with Electron's names in title case: Command+Alt+K. Every reader and
// writer of a shortcut goes through Normalize, so "cmd+option+k" and "Command+Alt+K" name one binding.
const (
	Command = "Command"
	Control = "Control"
	Alt     = "Alt"
	Shift   = "Shift"
	Escape  = "Escape"
	Enter   = "Enter"
	Space   = "Space"
	// CommandOrControl is Command on macOS and Control elsewhere; the binding plan resolves it per platform.
	CommandOrControl = "CommandOrControl"
)

var aliases = map[string]string{
	"commandorcontrol": CommandOrControl,
	"cmdorctrl":        CommandOrControl,
	"cmd":              Command,
	"command":          Command,
	"⌘":                Command,
	"ctrl":             Control,
	"control":          Control,
	"^":                Control,
	"option":           Alt,
	"opt":              Alt,
	"alt":              Alt,
	"⌥":                Alt,
	"shift":            Shift,
	"⇧":                Shift,
	"enter":            Enter,
	"return":           Enter,
	"↵":                Enter,
	"backspace":        "Backspace",
	"esc":              Escape,
	"escape":           Escape,
	"space":            Space,
}

// Normalize returns accelerator in canonical form: parts trimmed, aliases resolved, single letters upper-cased and
// the parts joined with "+". A blank accelerator normalizes to "".
func Normalize(accelerator string) string {
	var parts []string
	for _, part := range strings.Split(accelerator, "+") {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, normalizePart(part))
		}
	}
	return strings.Join(parts, "+")
}

// Parts returns the canonical parts of accelerator: its modifiers, then its key.
func Parts(accelerator string) []string {
	normalized := Normalize(accelerator)
	if normalized == "" {
		return nil
	}
	return strings.Split(normalized, "+")
}

func normalizePart(part string) string {
	if canonical, ok := aliases[strings.ToLower(part)]; ok {
		return canonical
	}
	if len(part) == 1 {
		return strings.ToUpper(part)
	}
	return part
}
