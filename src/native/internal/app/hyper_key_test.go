package app

import (
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

func TestHyperKeyIsTheDefaultWithoutAConfigFile(t *testing.T) {
	p := goldenPalette(t)
	if got := p.hyperKey(); got != shortcuts.DefaultHyperKey {
		t.Fatalf("hyper key = %q, want %q", got, shortcuts.DefaultHyperKey)
	}
}

func TestHyperKeyReadsTheOneXSettingFromConfigJSON(t *testing.T) {
	p, files := sharedConfigApp(t)
	if err := files.WriteFile(sharedFixtureConfig, []byte(oneXSharedConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := p.hyperKey(); got != "Command+Control+Alt" {
		t.Fatalf("hyper key = %q, want the 1.x value Command+Control+Alt", got)
	}
	if got := p.shortcutLabelOf("Command+Control+Alt+V"); got != "✦V" {
		t.Fatalf("label = %q, want ✦V", got)
	}
}

// TestHyperRecorderSavesTheModifiersHeldToConfigJSON holds Command, Control and Alt with no key, then saves with Enter.
func TestHyperRecorderSavesTheModifiersHeldToConfigJSON(t *testing.T) {
	p, files := sharedConfigApp(t)
	goldenOpen(p, p.hyperRecorderView())
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	tt.Key(ui.Super|ui.Ctrl|ui.Alt, 0)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	data, err := files.ReadFile(sharedFixtureConfig)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"hyperKey": "Command+Control+Alt"`) {
		t.Fatalf("config.json does not hold the hyper key:\n%s", data)
	}
	if got := p.hyperKey(); got != "Command+Control+Alt" {
		t.Fatalf("hyper key after save = %q", got)
	}
	if p.shortcutsManager().Suspended() {
		t.Fatal("saving left the bindings released")
	}
}

func TestHyperRecorderRefusesToSaveNothing(t *testing.T) {
	p, files := sharedConfigApp(t)
	goldenOpen(p, p.hyperRecorderView())
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if _, err := files.ReadFile(sharedFixtureConfig); err == nil {
		t.Fatal("saving nothing wrote config.json")
	}
	if got := p.hyperKey(); got != shortcuts.DefaultHyperKey {
		t.Fatalf("hyper key = %q, want the default", got)
	}
}

// TestHyperKeyLabelsKeepTheRealModifiersForMatching binds a hyper combination and checks the OS gets the real keys.
func TestHyperKeyLabelsKeepTheRealModifiersForMatching(t *testing.T) {
	p, files := sharedConfigApp(t)
	if err := files.WriteFile(sharedFixtureConfig, []byte(oneXSharedConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	mustSetShortcut(t, p.shortcutsManager(), shortcuts.CommandClipboardHistory, "Command+Control+Alt+K")
	if got := entryOf(t, p, shortcuts.CommandClipboardHistory).Accelerator; got != "Command+Control+Alt+K" {
		t.Fatalf("stored accelerator = %q, want the real modifiers", got)
	}
	registered := p.ports.Hotkeys.(*fake.Hotkeys).Registered()
	if !slices.Contains(registered, "Command+Control+Alt+K") {
		t.Fatalf("the OS was not asked for the real combination; registered %q", registered)
	}
}

func TestKeyboardShortcutsRowShowsTheHyperSparkle(t *testing.T) {
	p, files := sharedConfigApp(t)
	if err := files.WriteFile(sharedFixtureConfig, []byte(oneXSharedConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	row := p.shortcutRow(entryOf(t, p, "command:dictate"))
	if row.Accessory != "✦V" {
		t.Fatalf("dictation row accessory = %q, want ✦V", row.Accessory)
	}
}

func TestSettingsListShowsTheHyperKeyRow(t *testing.T) {
	p := goldenPalette(t)
	for _, row := range p.settingsView().Search("") {
		if row.Title != hyperKeyTitle {
			continue
		}
		if want := shortcuts.Label(shortcuts.DefaultHyperKey, runtime.GOOS); row.Accessory != want || row.Subtitle != hyperKeySubtitle {
			t.Fatalf("hyper row = %q / %q, want %q / %q", row.Accessory, row.Subtitle, want, hyperKeySubtitle)
		}
		return
	}
	t.Fatal("the settings list has no Hyper Key row")
}
