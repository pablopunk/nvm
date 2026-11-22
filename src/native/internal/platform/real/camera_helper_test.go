package real

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const cameraDevicesEnvelope = `{"ok":true,"code":"ok","data":[` +
	`{"id":"0x1600","title":"HP Display Camera","default":true,"kind":"external"},` +
	`{"id":"6C70","title":"","default":false,"kind":"built-in"},` +
	`{"id":"1705","title":"iPhone Camera","default":false,"kind":"continuity"},` +
	`{"id":"","title":"No ID","default":false,"kind":"external"},` +
	`{"id":"obs","title":"OBS Virtual Camera","default":false,"kind":"some-future-kind"}]}`

func TestCameraDevicesReadTheHelperListAndItsKinds(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script("helper", fake.HelperScript{Stdout: []byte(cameraDevicesEnvelope)})
	devices, err := macCameras{helper: helper, binary: "helper"}.Devices()
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	want := []platform.CameraDevice{
		{ID: "0x1600", Title: "HP Display Camera", Default: true, Kind: platform.CameraExternal},
		{ID: "6C70", Title: "6C70", Kind: platform.CameraBuiltIn},
		{ID: "1705", Title: "iPhone Camera", Kind: platform.CameraContinuity},
		{ID: "obs", Title: "OBS Virtual Camera", Kind: platform.CameraOther},
	}
	if !slices.Equal(devices, want) {
		t.Fatalf("devices = %+v, want %+v", devices, want)
	}
	if calls := helper.Calls(); len(calls) != 1 || !slices.Equal(calls[0].Args, []string{"camera", "devices"}) {
		t.Fatalf("helper calls = %+v, want one `camera devices`", calls)
	}
}

func TestCameraDevicesReportAHelperRefusal(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script("helper", fake.HelperScript{Stdout: []byte(`{"ok":false,"code":"failed","message":"no AVFoundation"}`)})
	if _, err := (macCameras{helper: helper, binary: "helper"}).Devices(); err == nil {
		t.Fatal("Devices succeeded on a refusal")
	}
}
