package shortcuts

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/portable"
)

const oneXShortcuts = `{
  "version": 1,
  "settings": {
    "paletteHotkey": "Command+Space",
    "hyperKey": "Command+Control+Alt"
  },
  "shortcuts": {
    "extension-root:nevermind.apps:app:/Applications/Slack.app": "Command+Control+Alt+D",
    "extension-root:nevermind.dictation:dictation": "Command+Control+Alt+V",
    "extension-action:window-management:layout.left-half": "Command+Control+Alt+Shift+H",
    "extension:nevermind.emoji-symbols:emoji-symbols": "Command+Shift+E",
    "extension-action:window-management:layout.focus-right": "Command+Control+Alt+L",
    "extension-action:other-extension:thing": "Command+Alt+T",
    "extension-action:window-management:layout.restore": null
  },
  "ai": {
    "defaultModel": "smart"
  }
}
`

func TestOneXKeysBindAsTheirTwoOhActions(t *testing.T) {
	tests := map[string]string{
		"extension-root:nevermind.apps:app:/Applications/Slack.app": "app:/Applications/Slack.app",
		"extension-action:window-management:layout.left-half":       "window.left-half",
		"extension-action:window-management:layout.focus-right":     "window.focus-right",
		"extension:nevermind.emoji-symbols:emoji-symbols":           CommandEmojiSymbols,
		"command:snippets": CommandSnippets,
	}
	for key, want := range tests {
		got, bound := ActionIDOf(key)
		if !bound || got != want {
			t.Errorf("ActionIDOf(%q) = %q, %v; want %q", key, got, bound, want)
		}
		if back := PortableKeyOf(got); back != key {
			t.Errorf("PortableKeyOf(%q) = %q; want %q", got, back, key)
		}
	}
	if _, bound := ActionIDOf("extension-action:other-extension:thing"); bound {
		t.Error("an action of another 1.x extension is bound")
	}
}

func TestPortableDocumentLoadsOneXShortcutsAndDisabledMarkers(t *testing.T) {
	files := fake.NewFS()
	if err := files.MkdirAll("/home/owner/.config/nevermind", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile("/home/owner/.config/nevermind/config.json", []byte(oneXShortcuts), 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := NewPortableDocument(portable.Open(files, "/home/owner/.config/nevermind/config.json")).Load()
	if err != nil {
		t.Fatal(err)
	}
	if doc.Palette != "Command+Space" {
		t.Errorf("palette = %q", doc.Palette)
	}
	if got := doc.Actions["app:/Applications/Slack.app"]; got != "Command+Control+Alt+D" {
		t.Errorf("app shortcut = %q", got)
	}
	if got := doc.Actions["window.left-half"]; got != "Command+Control+Alt+Shift+H" {
		t.Errorf("layout shortcut = %q", got)
	}
	if !doc.Disabled["window.restore"] {
		t.Error("a null shortcut is not a disabled marker")
	}
	if len(doc.Actions) != 5 {
		t.Errorf("actions = %v; the other extension's key must not be bound", doc.Actions)
	}
}

func TestPortableDocumentWritesOnlyTheKeysItChangesAndKeepsUnknownOnes(t *testing.T) {
	files := fake.NewFS()
	if err := files.MkdirAll("/home/owner/.config/nevermind", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile("/home/owner/.config/nevermind/config.json", []byte(oneXShortcuts), 0o600); err != nil {
		t.Fatal(err)
	}
	store := portable.Open(files, "/home/owner/.config/nevermind/config.json")
	persistence := NewPortableDocument(store)
	if err := persistence.Update(func(d *Document) {
		delete(d.Actions, "window.left-half")
		d.Actions["window.center"] = "Command+Alt+C"
		d.Disabled["window.restore"] = false
		d.Palette = "Alt+Space"
	}); err != nil {
		t.Fatal(err)
	}
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := doc.String("shortcuts", "extension-action:other-extension:thing"); got != "Command+Alt+T" {
		t.Errorf("an unknown 1.x shortcut was lost: %q", got)
	}
	if got, _ := doc.String("settings", "hyperKey"); got != "Command+Control+Alt" {
		t.Errorf("hyperKey was lost: %q", got)
	}
	if got, _ := doc.String("ai", "defaultModel"); got != "smart" {
		t.Errorf("ai.defaultModel was lost: %q", got)
	}
	if _, ok := doc.Value("shortcuts", "extension-action:window-management:layout.left-half"); ok {
		t.Error("a removed shortcut is still in the file")
	}
	if got, _ := doc.String("shortcuts", "extension-action:window-management:layout.center"); got != "Command+Alt+C" {
		t.Errorf("center shortcut = %q", got)
	}
	if got, _ := doc.String("settings", "paletteHotkey"); got != "Alt+Space" {
		t.Errorf("palette = %q", got)
	}
}

func TestBlankPaletteInConfigStaysBlankWhenAnActionChanges(t *testing.T) {
	files := fake.NewFS()
	if err := files.MkdirAll("/home/owner/.config/nevermind", 0o755); err != nil {
		t.Fatal(err)
	}
	path := "/home/owner/.config/nevermind/config.json"
	if err := files.WriteFile(path, []byte("{\n  \"version\": 1,\n  \"settings\": {}\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := portable.Open(files, path)
	manager := NewManager(Setup{Doc: NewPortableDocument(store), Hotkeys: fake.NewHotkeys(), Clock: fake.NewClock(), GOOS: "linux"})
	manager.Start()
	if err := manager.SetAction("window.center", "Command+Alt+C"); err != nil {
		t.Fatal(err)
	}
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if palette, present := doc.String("settings", "paletteHotkey"); present {
		t.Fatalf("paletteHotkey = %q was written by an action change", palette)
	}
}
