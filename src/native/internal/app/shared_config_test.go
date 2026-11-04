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
