package real

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const (
	micAllowed       = `{"ok":true,"code":"ok","data":{"status":"allowed"}}`
	micDenied        = `{"ok":true,"code":"ok","data":{"status":"denied"}}`
	micNotDetermined = `{"ok":true,"code":"ok","data":{"status":"not-determined"}}`
	micDevices       = `{"ok":true,"code":"ok","data":[{"id":"mic-usb","title":"USB Mic","default":false},{"id":"mic-builtin","title":"","default":true}]}`
)

var micFormat = platform.AudioFormat{SampleRate: 16000, Channels: 1}

// newScriptedMicrophone returns the macOS microphone over a fake helper and a binary path that exists, so the stream
// can run a real program. The fake answers the helper's envelopes; streamScript is the program the stream runs.
func newScriptedMicrophone(t *testing.T, streamScript string, answers ...fake.HelperScript) (macMicrophone, *fake.Helper) {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "macos-selected-text")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"+streamScript+"\n"), 0o700); err != nil {
		t.Fatalf("write stream program: %v", err)
	}
	helper := fake.NewHelper()
	helper.Script(binary, answers...)
	return macMicrophone{helper: helper, binary: binary}, helper
}

func TestDevicesReadTheHelperListAndTitleAnUnnamedDevice(t *testing.T) {
	mic, _ := newScriptedMicrophone(t, "", fake.HelperScript{Stdout: []byte(micDevices)})
	devices, err := mic.Devices()
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	want := []platform.MicrophoneDevice{
		{ID: "mic-usb", Title: "USB Mic"},
		{ID: "mic-builtin", Title: "mic-builtin", Default: true},
	}
	if !slices.Equal(devices, want) {
		t.Fatalf("devices = %+v, want %+v", devices, want)
	}
}

func TestPermissionStatusWordsMapToPermissionStates(t *testing.T) {
	tests := map[string]platform.PermissionState{
		"allowed":        platform.PermissionAllowed,
		"denied":         platform.PermissionDenied,
		"not-determined": platform.PermissionNotDetermined,
		"restricted":     platform.PermissionUnknown,
		"":               platform.PermissionUnknown,
	}
	for status, want := range tests {
		if got := permissionStateOfMicrophone(status); got != want {
			t.Errorf("permissionStateOfMicrophone(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestStateAsksTheHelperWithoutPrompting(t *testing.T) {
	mic, helper := newScriptedMicrophone(t, "", fake.HelperScript{Stdout: []byte(micDenied)})
	state, err := mic.State()
	if err != nil || state != platform.PermissionDenied {
		t.Fatalf("State() = %v, %v; want denied", state, err)
	}
	if calls := helper.Calls(); len(calls) != 1 || !slices.Equal(calls[0].Args, []string{"microphone", "access"}) {
		t.Fatalf("helper calls = %+v, want one microphone access call", calls)
	}
}

func TestHelperErrorEnvelopesAndCrashesAreErrors(t *testing.T) {
	tests := map[string]fake.HelperScript{
		"unreadable": {Stdout: []byte("crash")},
		"error":      {Stdout: []byte(`{"ok":false,"code":"error","message":"no session"}`), ExitCode: 1},
		"not run":    {Err: errors.New("no such file")},
	}
	for name, script := range tests {
		t.Run(name, func(t *testing.T) {
			mic, _ := newScriptedMicrophone(t, "", script)
			if _, err := mic.State(); err == nil {
				t.Fatalf("State() error = nil, want a helper error")
			}
		})
	}
}

func TestOpenRefusedAccessIsDeniedAndStartsNoStream(t *testing.T) {
	mic, helper := newScriptedMicrophone(t, "echo started >&2", fake.HelperScript{Stdout: []byte(micDenied)})
	stream, err := mic.Open(context.Background(), "", micFormat)
	if !errors.Is(err, platform.ErrDenied) || stream != nil {
		t.Fatalf("Open() = %v, %v; want ErrDenied and no stream", stream, err)
	}
	if calls := helper.Calls(); len(calls) != 1 {
		t.Fatalf("helper calls = %d, want only the access check", len(calls))
	}
}

func TestOpenAsksOnceWhenTheMicrophoneIsUndecided(t *testing.T) {
	pcm := []byte{1, 0}
	pcmPath := writePCMFile(t, pcm)
	mic, helper := newScriptedMicrophone(t, "cat "+pcmPath,
		fake.HelperScript{Stdout: []byte(micNotDetermined)},
		fake.HelperScript{Stdout: []byte(micAllowed)},
	)
	stream, err := mic.Open(context.Background(), "", micFormat)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = stream.Close() }()
	calls := helper.Calls()
	if len(calls) != 2 || !slices.Equal(calls[1].Args, []string{"microphone", "request"}) {
		t.Fatalf("helper calls = %+v, want access then request", calls)
	}
}

func TestOpenMissingDeviceIsNotFound(t *testing.T) {
	mic, _ := newScriptedMicrophone(t, "", fake.HelperScript{Stdout: []byte(micAllowed)}, fake.HelperScript{Stdout: []byte(micDevices)})
	if _, err := mic.Open(context.Background(), "gone", micFormat); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("Open(gone) error = %v, want ErrNotFound", err)
	}
}

func TestOpenStreamsTheDeviceAndFormatTheHelperWasGiven(t *testing.T) {
	samples := []int16{0, 1, -1, 32767, -32768}
	raw := make([]byte, 2*len(samples))
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(raw[2*i:], uint16(sample))
	}
	argsPath := filepath.Join(t.TempDir(), "args.txt")
	mic, _ := newScriptedMicrophone(t, `echo "$*" > `+argsPath+`; cat `+writePCMFile(t, raw),
		fake.HelperScript{Stdout: []byte(micAllowed)},
		fake.HelperScript{Stdout: []byte(micDevices)},
	)
	stream, err := mic.Open(context.Background(), "mic-usb", micFormat)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got := readAllSamples(t, stream)
	if !slices.Equal(got, samples) {
		t.Fatalf("samples = %v, want %v", got, samples)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil || strings.TrimSpace(string(args)) != "microphone stream mic-usb 16000 1" {
		t.Fatalf("stream arguments = %q, %v; want the device and the format", args, err)
	}
}

func TestStreamEndsWithTheDeniedStderrAsErrDenied(t *testing.T) {
	mic, _ := newScriptedMicrophone(t, "echo 'denied: Microphone access is denied' >&2; exit 2", fake.HelperScript{Stdout: []byte(micAllowed)})
	stream, err := mic.Open(context.Background(), "", micFormat)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = stream.Close() }()
	_, readErr := stream.Read(make([]int16, 4))
	if !errors.Is(readErr, platform.ErrDenied) {
		t.Fatalf("Read error = %v, want ErrDenied", readErr)
	}
}

func TestStreamStderrClassifiesTheFailure(t *testing.T) {
	if err := macStreamError("not-found: No microphone was found"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("not-found error = %v", err)
	}
	err := macStreamError("error: the session stopped")
	if err == nil || errors.Is(err, platform.ErrDenied) || !strings.Contains(err.Error(), "the session stopped") {
		t.Fatalf("other error = %v, want a plain failure carrying the message", err)
	}
}

func TestEmptyStreamEndsWithEOF(t *testing.T) {
	mic, _ := newScriptedMicrophone(t, "true", fake.HelperScript{Stdout: []byte(micAllowed)})
	stream, err := mic.Open(context.Background(), "", micFormat)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = stream.Close() }()
	if _, err := stream.Read(make([]int16, 4)); !errors.Is(err, io.EOF) {
		t.Fatalf("Read error = %v, want io.EOF", err)
	}
}

func writePCMFile(t *testing.T, raw []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pcm.raw")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write pcm: %v", err)
	}
	return path
}

func readAllSamples(t *testing.T, stream platform.MicrophoneStream) []int16 {
	t.Helper()
	defer func() { _ = stream.Close() }()
	var got []int16
	buf := make([]int16, 3)
	for {
		n, err := stream.Read(buf)
		got = append(got, buf[:n]...)
		if errors.Is(err, io.EOF) {
			return got
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
	}
}
