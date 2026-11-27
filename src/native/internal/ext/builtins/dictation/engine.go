package dictation

import (
	"fmt"
	"math"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// deviceModelDescription says what choosing the device means, and where its model stands now.
func deviceModelDescription(model api.DictationDeviceModel) string {
	about := "On this device is free, private and works offline, after a one-time " + sizeText(model.SizeBytes) +
		" download. " + model.Credit + "."
	switch model.Status {
	case api.DictationModelReady:
		return "The on-device model is downloaded. " + about
	case api.DictationModelDownloading:
		return fmt.Sprintf("Downloading the on-device model: %d%%. ", percentOf(model.Progress)) + about
	case api.DictationModelFailed:
		return "The last download failed (" + model.Error + "); download it again to resume. " + about
	}
	return about
}

// engineOrCloud reads an unset engine as the cloud, the engine dictation used before the choice existed.
func engineOrCloud(engine api.DictationEngine) api.DictationEngine {
	if engine == api.DictationEngineDevice {
		return engine
	}
	return api.DictationEngineCloud
}

// deviceModelNotice is what a dictation on the device says when its model is not ready, after starting the download
// when none runs. An empty notice means the model is ready.
func deviceModelNotice(ctx api.Context) string {
	model, err := ctx.Dictation().DeviceModel()
	if err != nil {
		return err.Error()
	}
	switch model.Status {
	case api.DictationModelReady:
		return ""
	case api.DictationModelDownloading:
		return fmt.Sprintf("the on-device model is downloading (%d%%)", percentOf(model.Progress))
	}
	if err := ctx.Dictation().InstallDeviceModel(); err != nil {
		return err.Error()
	}
	return "the on-device model is downloading (" + sizeText(model.SizeBytes) + ")"
}

// installNotice starts the model's download when the settings choose the device and it is not there yet, and returns
// the toast that says so.
func installNotice(ctx api.Context, engine api.DictationEngine) *api.Toast {
	if engine != api.DictationEngineDevice {
		return nil
	}
	model, err := ctx.Dictation().DeviceModel()
	if err != nil || model.Status == api.DictationModelReady || model.Status == api.DictationModelDownloading {
		return nil
	}
	if err := ctx.Dictation().InstallDeviceModel(); err != nil {
		return &api.Toast{Message: err.Error(), Tone: "error"}
	}
	return &api.Toast{Message: "Downloading the on-device model (" + sizeText(model.SizeBytes) + ")"}
}

// retryEngine is the engine a history retry runs on: the chosen one, unless that is the device without its model.
func retryEngine(ctx api.Context, chosen api.DictationEngine) api.DictationEngine {
	if engineOrCloud(chosen) == api.DictationEngineDevice {
		if model, err := ctx.Dictation().DeviceModel(); err == nil && model.Status == api.DictationModelReady {
			return api.DictationEngineDevice
		}
	}
	return api.DictationEngineCloud
}

func percentOf(progress float64) int {
	return int(math.Floor(min(1, max(0, progress)) * 100))
}

// sizeText is a download size in whole megabytes.
func sizeText(bytes int64) string {
	return fmt.Sprintf("%d MB", int(math.Round(float64(bytes)/1e6)))
}
