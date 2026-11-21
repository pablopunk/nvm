package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestKeyNameSpellsTheKeysAShortcutNames(t *testing.T) {
	for key, want := range map[ui.Key]string{
		ui.KeyK: "K", ui.KeyZ: "Z", ui.Key0: "0", ui.Key9: "9",
		ui.KeySpace: "Space", ui.KeyEnter: "Enter", ui.KeyUp: "Up", ui.KeyF5: "F5", ui.KeyComma: ",",
	} {
		if got := KeyName(key); got != want {
			t.Errorf("KeyName(%v) = %q, want %q", key, got, want)
		}
	}
	if got := KeyName(ui.KeyUnknown); got != "" {
		t.Errorf("KeyName(unknown) = %q, want empty", got)
	}
}
