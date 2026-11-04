package extview

import (
	"slices"
	"testing"
)

// engineRig is a camera engine with the page, the system and the memory replaced by recorders.
type engineRig struct {
	engine  *cameraEngine
	sent    []cameraCommand
	forgets int
	access  string
	changes int
}

func newEngineRig(access string) *engineRig {
	rig := &engineRig{access: access}
	rig.engine = &cameraEngine{
		send: func(command any) {
			rig.sent = append(rig.sent, command.(cameraCommand))
		},
		access:  func() string { return rig.access },
		forget:  func() { rig.forgets++ },
		onState: func() { rig.changes++ },
	}
	rig.engine.state.Status = cameraIdle
	return rig
}

// starts returns the device of every start the engine sent, in order; "" is the default camera.
func (r *engineRig) starts() []string {
	var devices []string
	for _, command := range r.sent {
		if command.Op == "start" {
			devices = append(devices, command.DeviceID)
		}
	}
	return devices
}

func (r *engineRig) ops() []string {
	ops := make([]string, 0, len(r.sent))
	for _, command := range r.sent {
		ops = append(ops, command.Op)
	}
	return ops
}

func live(id int, deviceID string) cameraEvent {
	return cameraEvent{Event: "live", ID: id, DeviceID: deviceID, Label: "Camera " + deviceID}
}

func failed(id int, name string) cameraEvent {
	return cameraEvent{Event: "error", ID: id, Name: name}
}

// TestStopWhilePendingDiscardsTheStreamThatArrivesLater proves a stream answering after Stop changes nothing.
func TestStopWhilePendingDiscardsTheStreamThatArrivesLater(t *testing.T) {
	rig := newEngineRig("")
	rig.engine.Start(cameraSelection{Source: cameraSourceDefault})
	rig.engine.Stop()
	rig.engine.Handle(live(1, "front"))
	if rig.engine.state.Status != cameraIdle {
		t.Fatalf("a stream after Stop showed %q, want paused", rig.engine.state.Status)
	}
}

// TestRestartReleasesThePreviousStreamBeforeTheNextStarts proves each start stops what runs before it starts.
func TestRestartReleasesThePreviousStreamBeforeTheNextStarts(t *testing.T) {
	rig := newEngineRig("")
	rig.engine.Start(cameraSelection{Source: cameraSourceDefault})
	rig.engine.Start(cameraSelection{DeviceID: "back", Source: cameraSourceExplicit})
	if want := []string{"stop", "start", "stop", "start"}; !slices.Equal(rig.ops(), want) {
		t.Fatalf("ops = %v, want %v", rig.ops(), want)
	}
}

// TestStaleSavedDeviceFallsBackToDefaultAndForgetsIt proves a saved camera the system no longer has is forgotten
// and the default camera is tried once.
func TestStaleSavedDeviceFallsBackToDefaultAndForgetsIt(t *testing.T) {
	rig := newEngineRig("")
	rig.engine.Start(cameraSelection{DeviceID: "gone", Source: cameraSourceSaved})
	rig.engine.Handle(failed(1, "NotFoundError"))
	if rig.forgets != 1 {
		t.Fatalf("forgets = %d, want 1", rig.forgets)
	}
	if want := []string{"gone", ""}; !slices.Equal(rig.starts(), want) {
		t.Fatalf("starts = %v, want %v", rig.starts(), want)
	}
	rig.engine.Handle(live(2, "front"))
	if rig.engine.state.Status != cameraLive {
		t.Fatalf("the fallback did not go live: %+v", rig.engine.state)
	}
}

// TestExplicitDeviceNeverFallsBackOrForgets proves a device the view named is never replaced by another camera.
func TestExplicitDeviceNeverFallsBackOrForgets(t *testing.T) {
	rig := newEngineRig("")
	rig.engine.Start(cameraSelection{DeviceID: "gone", Source: cameraSourceExplicit})
	rig.engine.Handle(failed(1, "NotFoundError"))
	if rig.forgets != 0 || len(rig.starts()) != 1 {
		t.Fatalf("explicit device fell back: forgets %d, starts %v", rig.forgets, rig.starts())
	}
	if rig.engine.state.Status != cameraError || rig.engine.state.Message != cameraErrorMessage("NotFoundError", "") {
		t.Fatalf("state = %+v, want the not-available error", rig.engine.state)
	}
}

// TestEndedTrackReportsDisconnectionOnce proves a track that ends on the live stream reports once and releases it.
func TestEndedTrackReportsDisconnectionOnce(t *testing.T) {
	rig := newEngineRig("")
	rig.engine.Start(cameraSelection{Source: cameraSourceDefault})
	rig.engine.Handle(live(1, "front"))
	rig.engine.Handle(cameraEvent{Event: "ended", ID: 1})
	rig.engine.Handle(cameraEvent{Event: "ended", ID: 1})
	if rig.engine.state.Message != cameraDisconnectedMessage {
		t.Fatalf("message = %q, want the disconnection copy", rig.engine.state.Message)
	}
	if stops := count(rig.ops(), "stop"); stops != 2 {
		t.Fatalf("stops = %d, want one for the start and one for the failure", stops)
	}
}

// TestPlaybackFailureOnlyTearsDownTheStreamThatFailed proves a playback report from a replaced stream is ignored.
func TestPlaybackFailureOnlyTearsDownTheStreamThatFailed(t *testing.T) {
	rig := newEngineRig("")
	rig.engine.Start(cameraSelection{Source: cameraSourceDefault})
	rig.engine.Start(cameraSelection{Source: cameraSourceDefault})
	rig.engine.Handle(cameraEvent{Event: "playbackError", ID: 1})
	if rig.engine.state.Status != cameraStarting {
		t.Fatalf("a replaced stream's failure changed the state to %q", rig.engine.state.Status)
	}
	rig.engine.Handle(live(2, "front"))
	rig.engine.Handle(cameraEvent{Event: "playbackError", ID: 2})
	if rig.engine.state.Message != cameraPlaybackMessage {
		t.Fatalf("message = %q, want the playback copy", rig.engine.state.Message)
	}
}

// TestDeniedSystemAccessStopsBeforeAnyDeviceIsRequested proves a denied camera never reaches the page.
func TestDeniedSystemAccessStopsBeforeAnyDeviceIsRequested(t *testing.T) {
	rig := newEngineRig("Camera access is denied in system privacy settings.")
	rig.engine.Start(cameraSelection{Source: cameraSourceDefault})
	if want := []string{"stop"}; !slices.Equal(rig.ops(), want) {
		t.Fatalf("ops = %v, want only the stop", rig.ops())
	}
	if rig.engine.state.Status != cameraError || rig.engine.state.Message == "" {
		t.Fatalf("state = %+v, want the denied error", rig.engine.state)
	}
}

// TestCameraErrorsMapToReadableCopy proves each DOM error name gets its copy, and an unknown one shows its message.
func TestCameraErrorsMapToReadableCopy(t *testing.T) {
	cases := map[string]string{
		"NotAllowedError":        "Camera access is denied. Allow Nevermind in system privacy settings.",
		"NotReadableError":       "The camera is busy. Close other apps using it, then reopen this view.",
		"NotFoundError":          "The camera is not available. Check that it is connected, or choose another camera.",
		"OverconstrainedError":   "The camera is not available. Check that it is connected, or choose another camera.",
		"NotSupportedError":      "This system web view cannot use the camera.",
		"AbortError":             "Camera unavailable.",
		"TypeError":              "Camera unavailable.",
	}
	for name, want := range cases {
		if got := cameraErrorMessage(name, ""); got != want {
			t.Errorf("cameraErrorMessage(%q) = %q, want %q", name, got, want)
		}
	}
	if got := cameraErrorMessage("SecurityError", "blocked by policy"); got != "blocked by policy" {
		t.Errorf("unknown error with message = %q", got)
	}
}

// TestNextCameraDeviceWrapsAndRecoversFromUnknownIDs proves switching wraps at both ends, starts from the edge when
// the current camera is unknown, and does nothing with a single camera.
func TestNextCameraDeviceWrapsAndRecoversFromUnknownIDs(t *testing.T) {
	devices := []cameraDevice{{DeviceID: "a"}, {DeviceID: "b"}, {DeviceID: "c"}}
	cases := []struct {
		current, direction, want string
	}{
		{"a", "next", "b"},
		{"c", "next", "a"},
		{"a", "previous", "c"},
		{"b", "previous", "a"},
		{"missing", "next", "a"},
		{"missing", "previous", "c"},
		{"", "next", "a"},
	}
	for _, c := range cases {
		if got := nextCameraDeviceID(devices, c.current, c.direction); got != c.want {
			t.Errorf("nextCameraDeviceID(%q, %q) = %q, want %q", c.current, c.direction, got, c.want)
		}
	}
	if got := nextCameraDeviceID(devices[:1], "a", "next"); got != "" {
		t.Errorf("one camera switched to %q", got)
	}
}

// TestPlayingReportShowsLive proves the status moves from starting to live when the first frame plays.
func TestPlayingReportShowsLive(t *testing.T) {
	rig := newEngineRig("")
	rig.engine.Start(cameraSelection{Source: cameraSourceDefault})
	rig.engine.Handle(live(1, "front"))
	if got := cameraStatusText(rig.engine.state); got != "Starting…" {
		t.Fatalf("status before the frame = %q", got)
	}
	rig.engine.Handle(cameraEvent{Event: "playing", ID: 1})
	if got := cameraStatusText(rig.engine.state); got != "Live" {
		t.Fatalf("status after the frame = %q", got)
	}
}

// TestDevicesReportsKeepTheListWithoutChangingTheStream proves the device list is kept whatever stream is running.
func TestDevicesReportsKeepTheListWithoutChangingTheStream(t *testing.T) {
	rig := newEngineRig("")
	rig.engine.Start(cameraSelection{Source: cameraSourceDefault})
	rig.engine.Handle(cameraEvent{Event: "devices", Devices: []cameraDevice{{DeviceID: "a", Label: "A"}, {DeviceID: "b"}}})
	if len(rig.engine.devices) != 2 || rig.engine.state.Status != cameraStarting {
		t.Fatalf("devices %v, state %+v", rig.engine.devices, rig.engine.state)
	}
}

func count(values []string, want string) int {
	n := 0
	for _, value := range values {
		if value == want {
			n++
		}
	}
	return n
}
