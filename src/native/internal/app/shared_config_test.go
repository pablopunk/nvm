package app

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/portable"
	"github.com/pablopunk/nvm/native/internal/update"
)

const sharedFixtureConfig = "/nvm-fixture/shared/nevermind/config.json"

// sharedConfigApp is a golden palette whose config store sits on a fake disk the test can read back.
func sharedConfigApp(t *testing.T) (*App, *fake.FS) {
	t.Helper()
	p := goldenPalette(t)
	files := fake.NewFS()
	if err := files.MkdirAll("/nvm-fixture/shared/nevermind", 0o755); err != nil {
		t.Fatal(err)
	}
	p.sharedConfig = portable.Open(files, sharedFixtureConfig)
	p.shortcutMgr = nil
	p.hyper = nil
	return p, files
}

func TestDiagnosticsSwitchIsSavedInConfigJSONForOneX(t *testing.T) {
	p, files := sharedConfigApp(t)
	p.toggleDiagnostics()
	data, err := files.ReadFile(sharedFixtureConfig)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"errorReporting": false`) {
		t.Fatalf("config.json does not hold the withdrawal:\n%s", data)
	}
	if !p.diagnosticsOff() {
		t.Fatal("the switch did not read back as off")
	}
}

func TestUpdateChannelIsSavedInConfigJSON(t *testing.T) {
	p, files := sharedConfigApp(t)
	channels := newUpdateChannels(p)
	if err := channels.SetChannel(update.ChannelNightly); err != nil {
		t.Fatal(err)
	}
	data, err := files.ReadFile(sharedFixtureConfig)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"updateChannel": "nightly"`) {
		t.Fatalf("config.json does not hold the channel:\n%s", data)
	}
	if channels.Channel() != update.ChannelNightly {
		t.Fatal("the channel did not read back as nightly")
	}
}

// oneXSharedConfig is a config.json 1.x wrote: the palette, and shortcuts for apps, dictation, emoji and window layouts.
const oneXSharedConfig = `{
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
    "extension-root:nevermind.dictation:dictation": "Command+Control+Alt+V",
    "extension:nevermind.emoji-symbols:emoji-symbols": "Command+Shift+E"
  },
  "ai": {
    "defaultModel": "smart"
  }
}
`

func TestOneXShortcutsAppearAsBoundRowsInTwoOh(t *testing.T) {
	p, files := sharedConfigApp(t)
	if err := files.WriteFile(sharedFixtureConfig, []byte(oneXSharedConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	accelerators := map[string]string{}
	for _, entry := range p.shortcutsManager().Entries() {
		accelerators[entry.ActionID] = entry.Accelerator
	}
	want := map[string]string{
		"palette":                     "Command+Space",
		"app:/Applications/Slack.app": "Command+Control+Alt+D",
		"command:dictate":             "Command+Control+Alt+V",
		"command:emoji-symbols":       "Command+Shift+E",
	}
	for actionID, accelerator := range want {
		if got := accelerators[actionID]; got != accelerator {
			t.Errorf("%s = %q; want %q", actionID, got, accelerator)
		}
	}
}
