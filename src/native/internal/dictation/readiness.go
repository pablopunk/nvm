package dictation

import (
	"regexp"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Readiness is the wait Bluetooth headsets need before the first words count. Their microphone takes a moment to start
// delivering sound, and a recording that begins too early loses the first words. The rules are the Electron app's: a
// frame with sound must be followed by a few more with sound in one unbroken stream, or the stream must stay silent for
// five seconds, at which point the microphone is taken as ready.
const (
	readinessMaxFrameGapMS = 250
	readinessSignalFrames  = 3
	readinessSilentMS      = 5000
)

// bluetoothLabel matches the names the OS gives Bluetooth audio devices.
var bluetoothLabel = regexp.MustCompile(`(?i)airp(od|ol)|bluetooth|beats|bose|jabra|headset`)

// NeedsBluetoothReadiness reports whether a device with this name needs the readiness wait. The Electron app also looks
// at the device's native sample rate; this build opens every device at 16 kHz, so the name decides alone.
func NeedsBluetoothReadiness(name string) bool {
	return bluetoothLabel.MatchString(name)
}

// readinessTracker counts the frames of one recording that carry sound. Times are in milliseconds of audio, so the
// tracker needs no clock.
type readinessTracker struct {
	started          bool
	lastFrameMS      float64
	streakStartMS    float64
	framesSinceSound int
}

// consume takes one frame at nowMS and reports whether the microphone is ready.
func (r *readinessTracker) consume(nowMS float64, hasSound bool) bool {
	switch {
	case !r.started || nowMS-r.lastFrameMS > readinessMaxFrameGapMS:
		r.started = true
		r.streakStartMS = nowMS
		r.framesSinceSound = 0
	case r.framesSinceSound > 0:
		r.framesSinceSound++
	}
	if hasSound && r.framesSinceSound == 0 {
		r.framesSinceSound = 1
	}
	r.lastFrameMS = nowMS
	return r.framesSinceSound >= readinessSignalFrames || nowMS-r.streakStartMS >= readinessSilentMS
}

// hasSound reports whether a frame holds any sample other than silence.
func hasSound(frame []int16) bool {
	for _, sample := range frame {
		if sample != 0 {
			return true
		}
	}
	return false
}

// DeviceTitle returns the name of the device deviceID, or of the system default when deviceID is "" or "default". It
// returns "" when the device is not listed.
func DeviceTitle(devices []platform.MicrophoneDevice, deviceID string) string {
	for _, device := range devices {
		if (deviceID == "" || deviceID == "default") && device.Default || device.ID == deviceID {
			return device.Title
		}
	}
	return ""
}
