package rigtest

import "testing"

func TestXdotoolKeysTranslatesAcceleratorsForTheRig(t *testing.T) {
	tests := []struct {
		accelerator string
		want        string
	}{
		{"Alt+Shift+Space", "alt+shift+space"},
		{"Esc", "Escape"},
		{"Enter", "Return"},
		{"Cmd+K", "super+k"},
		{"CmdOrCtrl+Shift+F5", "ctrl+shift+F5"},
		{"Down", "Down"},
		{"a", "a"},
	}
	for _, tt := range tests {
		got, err := XdotoolKeys(tt.accelerator)
		if err != nil {
			t.Fatalf("XdotoolKeys(%q): %v", tt.accelerator, err)
		}
		if got != tt.want {
			t.Errorf("XdotoolKeys(%q) = %q, want %q", tt.accelerator, got, tt.want)
		}
	}
}

func TestXdotoolKeysRejectsUnknownKeysInsteadOfGuessing(t *testing.T) {
	for _, accelerator := range []string{"", "Hyper+Space", "Ctrl+"} {
		if got, err := XdotoolKeys(accelerator); err == nil {
			t.Errorf("XdotoolKeys(%q) = %q, want an error", accelerator, got)
		}
	}
}
