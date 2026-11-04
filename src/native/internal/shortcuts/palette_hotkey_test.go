package shortcuts

import "testing"

func TestPaletteHotkeyBindsTheChosenKey(t *testing.T) {
	hotkeys := newRecordingHotkeys()
	palette := NewPaletteHotkey(hotkeys, "darwin", func() {})
	status := palette.Bind("Alt+Shift+Space")
	if !status.Registered || status.Recovery != "" {
		t.Fatalf("status = %+v", status)
	}
	if !palette.Reserved("Alt+Shift+Space") {
		t.Fatal("the chosen key is not reserved")
	}
}

func TestPaletteHotkeyRebindReleasesThePreviousKey(t *testing.T) {
	hotkeys := newRecordingHotkeys()
	palette := NewPaletteHotkey(hotkeys, "darwin", func() {})
	palette.Bind("Alt+Shift+Space")
	palette.Bind("Control+Shift+Space")
	if len(hotkeys.unregistered) != 1 || hotkeys.unregistered[0] != "Alt+Shift+Space" {
		t.Fatalf("unregistered = %v", hotkeys.unregistered)
	}
	if palette.Reserved("Alt+Shift+Space") {
		t.Fatal("the released key is still reserved")
	}
}

func TestPaletteHotkeyOnLinuxBindsARecoveryKeyWhenTheChosenOneIsTaken(t *testing.T) {
	hotkeys := newRecordingHotkeys("Alt+Shift+Space")
	palette := NewPaletteHotkey(hotkeys, "linux", func() {})
	status := palette.Bind("Alt+Shift+Space")
	if status.Registered || status.Recovery != "Control+Alt+K" {
		t.Fatalf("status = %+v, want the first recovery key", status)
	}
	if !palette.Reserved("Control+Alt+K") {
		t.Fatal("the recovery key is not reserved")
	}
}

func TestPaletteHotkeyOffLinuxHasNoRecoveryKey(t *testing.T) {
	hotkeys := newRecordingHotkeys("Alt+Shift+Space")
	status := NewPaletteHotkey(hotkeys, "windows", func() {}).Bind("Alt+Shift+Space")
	if status.Registered || status.Recovery != "" {
		t.Fatalf("status = %+v", status)
	}
}
