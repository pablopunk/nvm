package dictation

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestOnDeviceDictationStartsWithoutAnAccountOnceTheModelIsReady(t *testing.T) {
	ctx := newContext()
	ctx.dictation.available = false
	ctx.dictation.model.Status = api.DictationModelReady
	ctx.settings.choose(t, map[string]any{SettingDeviceID: "default", SettingEngine: "device"})
	dictate(t, New(Legacy{}), ctx)
	if len(ctx.dictation.starts) != 1 || ctx.dictation.starts[0].Engine != api.DictationEngineDevice {
		t.Fatalf("starts = %+v; want one dictation on the device", ctx.dictation.starts)
	}
}

func TestOnDeviceDictationWithoutItsModelStartsTheDownloadAndSaysSo(t *testing.T) {
	ctx := newContext()
	ctx.settings.choose(t, map[string]any{SettingDeviceID: "default", SettingEngine: "device"})
	dictate(t, New(Legacy{}), ctx)
	if len(ctx.dictation.starts) != 0 {
		t.Fatal("a dictation started on a model that is not downloaded")
	}
	if ctx.dictation.installs != 1 {
		t.Fatalf("installs = %d; want the download started", ctx.dictation.installs)
	}
	if last := ctx.indicators.last(); !strings.Contains(last.Subtitle, "downloading") {
		t.Fatalf("indicator = %q; want it to say the model is downloading", last.Subtitle)
	}
	dictate(t, New(Legacy{}), ctx)
	if ctx.dictation.installs != 1 {
		t.Fatalf("installs = %d; a running download was started again", ctx.dictation.installs)
	}
}

func TestTheSettingsPageOffersTheModelDownloadUntilItIsReady(t *testing.T) {
	ctx := newContext()
	if row := engineRow(api.DictationDeviceModel{Status: api.DictationModelReady}); len(row.Also) != 0 {
		t.Fatalf("engine row %+v offers a download of a ready model", row)
	}
	row := engineRow(api.DictationDeviceModel{})
	if len(row.Also) != 1 || row.Also[0].Action == nil || row.Also[0].Action.Ref.ID != refInstallModel {
		t.Fatalf("engine row %+v; want the download offered", row)
	}
	result := run(t, New(Legacy{}), ctx, refInstallModel, nil, nil)
	if ctx.dictation.installs != 1 || result.Toast == nil || !strings.Contains(result.Toast.Message, "670 MB") {
		t.Fatalf("installs = %d, toast = %+v; want the download started and announced with its size", ctx.dictation.installs, result.Toast)
	}
}

func TestHistoryRetriesOnTheDeviceOnlyWhenItsModelIsReady(t *testing.T) {
	ctx := newContext()
	ctx.dictation.retryText = "recovered"
	ctx.settings.choose(t, map[string]any{SettingDeviceID: "default", SettingEngine: "device"})
	run(t, New(Legacy{}), ctx, refRetry, map[string]string{argID: "dictation-a"}, nil)
	ctx.dictation.model.Status = api.DictationModelReady
	run(t, New(Legacy{}), ctx, refRetry, map[string]string{argID: "dictation-b"}, nil)
	want := []api.DictationEngine{api.DictationEngineCloud, api.DictationEngineDevice}
	if len(ctx.dictation.retryEngines) != 2 || ctx.dictation.retryEngines[0] != want[0] || ctx.dictation.retryEngines[1] != want[1] {
		t.Fatalf("retry engines = %v; want %v", ctx.dictation.retryEngines, want)
	}
}
