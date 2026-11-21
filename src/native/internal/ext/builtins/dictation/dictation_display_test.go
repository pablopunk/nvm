package dictation

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestLoadingCommandsPublishesTheChosenMicrophoneTitle(t *testing.T) {
	ctx := newContext()
	ext := New(Legacy{Settings: &Settings{DeviceID: "mic-1"}})
	if _, err := ext.Commands(ctx); err != nil {
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

func TestSavingSettingsPublishesTheNewMicrophoneTitle(t *testing.T) {
	ctx := newContext()
	ctx.dictation.devices = append(ctx.dictation.devices, api.DictationDevice{ID: "mic-2", Title: "Desk Mic"})
	ext := New(Legacy{})
	form := api.FormValues{fieldDevice: api.TextValue("mic-2")}
	if result := run(t, ext, ctx, refSaveSettings, nil, form); result.Navigation != api.NavPop {
		t.Fatalf("navigation %q; want pop", result.Navigation)
	}
	if got := ctx.settings.displays[SettingDevice]; got != "Desk Mic" {
		t.Fatalf("published %q after the save; want the saved microphone's title", got)
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
