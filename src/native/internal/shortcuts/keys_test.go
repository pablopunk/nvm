package shortcuts

import "testing"

func TestCaptureNeedsAModifierExceptForFunctionKeysAndArrows(t *testing.T) {
	tests := []struct {
		name string
		mods Modifiers
		key  string
		want string
	}{
		{"command and letter", Modifiers{Command: true}, "K", "Command+K"},
		{"modifiers in canonical order", Modifiers{Shift: true, Alt: true, Control: true, Command: true}, "K", "Command+Control+Alt+Shift+K"},
		{"letter without a modifier", Modifiers{}, "K", ""},
		{"no key", Modifiers{Command: true}, "", ""},
		{"function key alone", Modifiers{}, "F5", "F5"},
		{"arrow alone", Modifiers{}, "Up", "Up"},
		{"a bare F is not a function key", Modifiers{}, "F", ""},
		{"space with alt", Modifiers{Alt: true}, "Space", "Alt+Space"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Capture(tt.mods, tt.key); got != tt.want {
				t.Fatalf("Capture = %q, want %q", got, tt.want)
			}
		})
	}
}
