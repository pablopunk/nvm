package extview

import "testing"

// hostListedRig is an engine rig whose cameras come from the host's list.
func hostListedRig() *engineRig {
	rig := newEngineRig("")
	rig.engine.SetHostDevices([]cameraDevice{
		{DeviceID: "built-in", Label: "FaceTime HD Camera"},
		{DeviceID: "usb-1", Label: "HP Display Camera"},
	})
	return rig
}

func lastStart(rig *engineRig) cameraCommand {
	for i := len(rig.sent) - 1; i >= 0; i-- {
		if rig.sent[i].Op == "start" {
			return rig.sent[i]
		}
	}
	return cameraCommand{}
}

func TestHostCameraIsOpenedByTheNameThePageKnows(t *testing.T) {
	rig := hostListedRig()
	rig.engine.Start(cameraSelection{DeviceID: "usb-1", Source: cameraSourceExplicit})
	start := lastStart(rig)
	if start.DeviceID != "" || start.Label != "HP Display Camera" {
		t.Fatalf("start %+v; want the camera's name and no page ID", start)
	}
}

func TestLiveHostCameraReportsTheHostID(t *testing.T) {
	rig := hostListedRig()
	rig.engine.Start(cameraSelection{DeviceID: "usb-1", Source: cameraSourceExplicit})
	rig.engine.Handle(cameraEvent{Event: "live", ID: rig.engine.generation, DeviceID: "page-hash", Label: "HP Display Camera"})
	if rig.engine.state.DeviceID != "usb-1" {
		t.Fatalf("live device %q; want the host ID usb-1", rig.engine.state.DeviceID)
	}
}

func TestExplicitHostCameraThatIsGoneSaysSoWithoutFallingBack(t *testing.T) {
	rig := hostListedRig()
	rig.engine.Start(cameraSelection{DeviceID: "unplugged", Source: cameraSourceExplicit})
	if rig.engine.state.Status != cameraError {
		t.Fatalf("status %q; want an error", rig.engine.state.Status)
	}
	if starts := rig.starts(); len(starts) != 0 {
		t.Fatalf("starts %q; an explicit camera never falls back", starts)
	}
}

func TestSavedHostCameraThatIsGoneFallsBackToTheDefault(t *testing.T) {
	rig := hostListedRig()
	rig.engine.Start(cameraSelection{DeviceID: "unplugged", Source: cameraSourceSaved})
	if rig.forgets != 1 {
		t.Fatalf("forgets %d; want the stale camera forgotten", rig.forgets)
	}
	if start := lastStart(rig); start.DeviceID != "" || start.Label != "" {
		t.Fatalf("start %+v; want the system default", start)
	}
}

func TestPageDeviceChangeAsksTheHostAgain(t *testing.T) {
	rig := hostListedRig()
	relisted := 0
	rig.engine.relist = func() { relisted++ }
	rig.engine.Handle(cameraEvent{Event: "devices", Devices: []cameraDevice{{DeviceID: "page-hash", Label: "HP Display Camera"}}})
	if relisted != 1 || rig.engine.devices[0].DeviceID != "built-in" {
		t.Fatalf("relisted %d, devices %+v; want the host list kept and asked again", relisted, rig.engine.devices)
	}
}
