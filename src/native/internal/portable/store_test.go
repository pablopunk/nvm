package portable

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const configPath = "/home/owner/.config/nevermind/config.json"

// oneXSample is a config.json as 1.x writes it: two-space indentation, a trailing newline, and keys it does not use.
const oneXSample = `{
  "version": 1,
  "settings": {
    "paletteHotkey": "Command+Space",
    "hyperKey": "Command+Control+Alt",
    "errorReporting": true,
    "updateChannel": "dev",
    "menuBarIconStyle": "bolt-outline"
  },
  "shortcuts": {
    "extension-root:nevermind.apps:app:/Applications/Slack.app": "Command+Control+Alt+D",
    "extension-action:window-management:layout.focus-right": "Command+Control+Alt+L",
    "extension-action:other:a.b": null
  },
  "ai": {
    "defaultModel": "smart"
  },
  "unknownSection": {
    "kept": [
      1.0,
      "<&>"
    ]
  }
}
`

func openWith(t *testing.T, files *fake.FS, text string) *Store {
	t.Helper()
	if err := files.MkdirAll("/home/owner/.config/nevermind", 0o755); err != nil {
		t.Fatal(err)
	}
	if text != "" {
		if err := files.WriteFile(configPath, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return Open(files, configPath)
}

func readText(t *testing.T, files *fake.FS, path string) string {
	t.Helper()
	data, err := files.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestUpdateChangesOnlyTheKeyItSetsAndKeepsEverythingElse(t *testing.T) {
	files := fake.NewFS()
	store := openWith(t, files, oneXSample)
	if err := store.Update(func(d Document) { d.SetValue("Command+Alt+Space", "settings", "paletteHotkey") }); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(oneXSample, `"Command+Space"`, `"Command+Alt+Space"`, 1)
	if got := readText(t, files, configPath); got != want {
		t.Errorf("rewritten file differs from the 1.x layout:\n%s\nwant:\n%s", got, want)
	}
}

func TestUpdateWithoutChangeLeavesTheFileUntouched(t *testing.T) {
	files := fake.NewFS()
	store := openWith(t, files, strings.TrimSuffix(oneXSample, "\n"))
	if err := store.Update(func(d Document) { d.SetValue("smart", "ai", "defaultModel") }); err != nil {
		t.Fatal(err)
	}
	if got := readText(t, files, configPath); got != strings.TrimSuffix(oneXSample, "\n") {
		t.Errorf("a no-op update rewrote the file: %q", got)
	}
}

func TestMissingFileIsCreatedWithVersionAndPrivateMode(t *testing.T) {
	files := fake.NewFS()
	store := openWith(t, files, "")
	if err := store.Update(func(d Document) { d.SetValue("Alt+Space", "settings", "paletteHotkey") }); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"version\": 1,\n  \"settings\": {\n    \"paletteHotkey\": \"Alt+Space\"\n  },\n  \"shortcuts\": {}\n}\n"
	if got := readText(t, files, configPath); got != want {
		t.Errorf("created file = %q; want %q", got, want)
	}
	info, err := files.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v; want 0600", info.Mode().Perm())
	}
}

func TestInvalidOrNewerFilesAreReadOnly(t *testing.T) {
	tests := map[string]error{
		"{not json":                         nil,
		`{"version":2,"settings":{}}`:       ErrUnsupportedVersion,
		`{"settings":{}}`:                   ErrUnsupportedVersion,
		`{"version":1,"shortcuts":[]}`:      nil,
		`{"version":1,"shortcuts":{"a":3}}`: nil,
	}
	for text, wantErr := range tests {
		files := fake.NewFS()
		store := openWith(t, files, text)
		if _, err := store.Load(); err == nil {
			t.Errorf("Load(%q) returned no error", text)
		} else if wantErr != nil && !errors.Is(err, wantErr) {
			t.Errorf("Load(%q) = %v; want %v", text, err, wantErr)
		}
		if err := store.Update(func(d Document) { d.SetValue("x", "settings", "paletteHotkey") }); err == nil {
			t.Errorf("Update(%q) wrote over an unusable file", text)
		}
		if got := readText(t, files, configPath); got != text {
			t.Errorf("unusable file was changed: %q", got)
		}
	}
}

func TestUpdateKeepsAnotherAppsChangesMadeAfterLoad(t *testing.T) {
	files := fake.NewFS()
	store := openWith(t, files, oneXSample)
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	external := strings.Replace(oneXSample, `"dev"`, `"nightly"`, 1)
	if err := files.WriteFile(configPath, []byte(external), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(d Document) { d.SetValue("Alt+Space", "settings", "paletteHotkey") }); err != nil {
		t.Fatal(err)
	}
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if channel, _ := doc.String("settings", "updateChannel"); channel != "nightly" {
		t.Errorf("another app's update channel was overwritten: %q", channel)
	}
	if palette, _ := doc.String("settings", "paletteHotkey"); palette != "Alt+Space" {
		t.Errorf("palette = %q", palette)
	}
}

func TestUpdateWritesThroughASymbolicLinkAndKeepsTheLink(t *testing.T) {
	files := fake.NewFS()
	if err := files.MkdirAll("/home/owner/.config/nevermind", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := files.MkdirAll("/home/owner/dotfiles", 0o755); err != nil {
		t.Fatal(err)
	}
	target := "/home/owner/dotfiles/config.json"
	if err := files.WriteFile(target, []byte(oneXSample), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := files.Symlink(target, configPath); err != nil {
		t.Fatal(err)
	}
	store := Open(files, configPath)
	if err := store.Update(func(d Document) { d.SetValue("Alt+Space", "settings", "paletteHotkey") }); err != nil {
		t.Fatal(err)
	}
	info, err := files.Lstat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Error("the link was replaced by a file")
	}
	if got := readText(t, files, target); !strings.Contains(got, `"Alt+Space"`) {
		t.Errorf("the link target was not updated: %s", got)
	}
}

func TestDanglingLinkIsNeverWritten(t *testing.T) {
	files := fake.NewFS()
	if err := files.MkdirAll("/home/owner/.config/nevermind", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := files.Symlink("/home/owner/dotfiles/missing.json", configPath); err != nil {
		t.Fatal(err)
	}
	store := Open(files, configPath)
	if err := store.Update(func(d Document) { d.SetValue("x", "settings", "paletteHotkey") }); !errors.Is(err, ErrDanglingLink) {
		t.Errorf("Update over a dangling link = %v; want ErrDanglingLink", err)
	}
}

func TestSettingsReadTheSharedKeysAndLeaveMissingOnesEmpty(t *testing.T) {
	doc, err := parseDocument([]byte(oneXSample))
	if err != nil {
		t.Fatal(err)
	}
	settings := doc.Settings()
	if settings.PaletteHotkey != "Command+Space" || settings.HyperKey == nil || *settings.HyperKey != "Command+Control+Alt" || settings.UpdateChannel != "dev" || settings.MenuBarIconStyle != "bolt-outline" || settings.AIDefaultModel != "smart" {
		t.Errorf("settings = %+v", settings)
	}
	if settings.ErrorReporting == nil || !*settings.ErrorReporting {
		t.Errorf("errorReporting = %v; want true", settings.ErrorReporting)
	}
	empty, err := parseDocument([]byte(`{"version":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := empty.Settings(); got.ErrorReporting != nil || got.HyperKey != nil || got.PaletteHotkey != "" {
		t.Errorf("missing keys read as %+v", got)
	}
}
