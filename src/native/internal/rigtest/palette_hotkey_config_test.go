package rigtest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/folders"
)

func writeSharedConfig(t *testing.T, home, body string) {
	t.Helper()
	dir := folders.Shared(home)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, folders.PortableConfigName), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPaletteHotkeyMissingForAHomeWithoutConfig(t *testing.T) {
	if !paletteHotkeyMissing(t.TempDir()) {
		t.Fatal("a HOME with no config.json has no palette shortcut, so the rig must write one")
	}
}

func TestPaletteHotkeyMissingKeepsAHotkeyALaunchLeft(t *testing.T) {
	home := t.TempDir()
	writeSharedConfig(t, home, `{"version":1,"settings":{"paletteHotkey":"Alt+Shift+Space"}}`)
	if paletteHotkeyMissing(home) {
		t.Fatal("a config that holds the palette shortcut must keep it on relaunch")
	}
}

func TestPaletteHotkeyMissingForAConfigWithoutOne(t *testing.T) {
	home := t.TempDir()
	writeSharedConfig(t, home, `{"version":1,"settings":{"hyperKey":"Cmd"}}`)
	if !paletteHotkeyMissing(home) {
		t.Fatal("a config that holds no palette shortcut must get one")
	}
}

func TestPaletteHotkeyMissingForAConfigThatIsNotJSON(t *testing.T) {
	home := t.TempDir()
	writeSharedConfig(t, home, `{"settings":`)
	if !paletteHotkeyMissing(home) {
		t.Fatal("an unreadable config must be treated as missing the shortcut")
	}
}
