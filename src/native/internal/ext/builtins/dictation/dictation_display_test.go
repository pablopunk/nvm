package dictation

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestLoadingCommandsPublishesTheChosenMicrophoneTitle(t *testing.T) {
	ctx := newContext()
	ctx.settings.choose(t, map[string]any{SettingDeviceID: "mic-1"})
	if _, err := New(Legacy{}).Commands(ctx); err != nil {
		t.Fatal(err)
	}
	if got := ctx.settings.displays[SettingDevice]; got != "USB Mic" {
		t.Fatalf("published %q; want the chosen microphone's title", got)
	}
}

func TestTheSystemMicrophoneIsPublishedByItsTitle(t *testing.T) {
	ctx := newContext()
	if _, err := New(Legacy{}).Commands(ctx); err != nil {
		t.Fatal(err)
	}
	if got := ctx.settings.displays[SettingDevice]; got != "USB Mic" {
		t.Fatalf("published %q for the system microphone; want the default device's title", got)
	}
}

func TestOpeningSettingsPublishesTheChosenMicrophoneTitle(t *testing.T) {
	ctx := newContext()
	ctx.dictation.devices = append(ctx.dictation.devices, api.DictationDevice{ID: "mic-2", Title: "Desk Mic"})
	ctx.settings.choose(t, map[string]any{SettingDeviceID: "mic-2"})
	run(t, New(Legacy{}), ctx, RefOpenSettings, nil, nil)
	if got := ctx.settings.displays[SettingDevice]; got != "Desk Mic" {
		t.Fatalf("published %q on opening the settings; want the chosen microphone's title", got)
	}
}

func TestAnUnlistedMicrophoneIsPublishedByItsID(t *testing.T) {
	if got := displayTitleOf(nil, "mic-gone"); got != "mic-gone" {
		t.Fatalf("title %q; want the id itself", got)
	}
	if got := displayTitleOf(nil, defaultDeviceID); got != systemDefaultTitle {
		t.Fatalf("title %q; want the system default", got)
	}
}
