package real

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/swifthelper"
)

// microphoneSubcommand is the helper's microphone command group: devices, access, request and stream.
const microphoneSubcommand = "microphone"

// microphoneRequestTimeout bounds the wait for the user to answer the system's microphone prompt.
const microphoneRequestTimeout = 60 * time.Second

// defaultMicrophoneID is the device ID that means the system default input.
const defaultMicrophoneID = "default"

// macMicrophone records through the Swift helper that ships with the app. `microphone devices|access|request` answer
// with the helper's envelope; `microphone stream <device> <rate> <channels>` writes signed 16-bit PCM to stdout until
// it is killed, and says why it stopped on stderr with a prefix the stream error mapping reads.
type macMicrophone struct {
	helper platform.Helper
	binary string
}

// microphoneDeviceWire is one entry of the helper's device list.
type microphoneDeviceWire struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Default bool   `json:"default"`
}

// Devices lists the input devices and marks the system default.
func (m macMicrophone) Devices() ([]platform.MicrophoneDevice, error) {
	ctx, cancel := context.WithTimeout(context.Background(), macosCallTimeout)
	defer cancel()
	var wire []microphoneDeviceWire
	if err := m.decode(ctx, &wire, "devices"); err != nil {
		return nil, err
	}
	devices := make([]platform.MicrophoneDevice, 0, len(wire))
	for _, device := range wire {
		title := device.Title
		if title == "" {
			title = device.ID
		}
		devices = append(devices, platform.MicrophoneDevice{ID: device.ID, Title: title, Default: device.Default})
	}
	return devices, nil
}

// Open checks the microphone permission (asking the system once when it has not been answered), checks the device
// exists, and starts the helper's stream. A refused permission is ErrDenied and a missing device is ErrNotFound.
func (m macMicrophone) Open(ctx context.Context, deviceID string, format platform.AudioFormat) (platform.MicrophoneStream, error) {
	if err := m.ensureAccess(); err != nil {
		return nil, err
	}
	device := deviceID
	if device == "" {
		device = defaultMicrophoneID
	}
	if device != defaultMicrophoneID {
		if err := m.ensureDevice(device); err != nil {
			return nil, err
		}
	}
	args := []string{microphoneSubcommand, "stream", device, strconv.Itoa(format.SampleRate), strconv.Itoa(format.Channels)}
	cmd := exec.CommandContext(ctx, m.binary, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("the microphone stream could not start: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("the microphone stream could not start: %w", err)
	}
	return newPCMProcessStream(cmd, stdout, &stderr, macStreamError), nil
}

// State reports the microphone permission without a prompt.
func (m macMicrophone) State() (platform.PermissionState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), macosCallTimeout)
	defer cancel()
	return m.accessStatus(ctx, "access")
}

// Request shows the system's microphone prompt when the user has not answered it, and reports the answer.
func (m macMicrophone) Request() (platform.PermissionState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), microphoneRequestTimeout)
	defer cancel()
	return m.accessStatus(ctx, "request")
}

// ensureAccess returns nil only when the microphone is allowed, asking the system first when it is undecided.
func (m macMicrophone) ensureAccess() error {
	state, err := m.State()
	if err != nil {
		return err
	}
	if state == platform.PermissionNotDetermined {
		if state, err = m.Request(); err != nil {
			return err
		}
	}
	if state != platform.PermissionAllowed {
		return platform.ErrDenied
	}
	return nil
}

// ensureDevice returns platform.ErrNotFound when no listed input device has the ID.
func (m macMicrophone) ensureDevice(id string) error {
	devices, err := m.Devices()
	if err != nil {
		return err
	}
	for _, device := range devices {
		if device.ID == id {
			return nil
		}
	}
	return platform.ErrNotFound
}

func (m macMicrophone) accessStatus(ctx context.Context, subcommand string) (platform.PermissionState, error) {
	var answer struct {
		Status string `json:"status"`
	}
	if err := m.decode(ctx, &answer, subcommand); err != nil {
		return platform.PermissionUnknown, err
	}
	return permissionStateOfMicrophone(answer.Status), nil
}

// permissionStateOfMicrophone maps the helper's status words to the permission states the settings view shows.
func permissionStateOfMicrophone(status string) platform.PermissionState {
	switch status {
	case "allowed":
		return platform.PermissionAllowed
	case "denied":
		return platform.PermissionDenied
	case "not-determined":
		return platform.PermissionNotDetermined
	}
	return platform.PermissionUnknown
}

// decode runs one microphone subcommand and reads the data of its ok envelope into dst.
func (m macMicrophone) decode(ctx context.Context, dst any, args ...string) error {
	callCtx, cancel := context.WithTimeout(ctx, macosCallTimeout)
	defer cancel()
	result, err := m.helper.Run(callCtx, m.binary, append([]string{microphoneSubcommand}, args...), nil)
	if err != nil {
		return fmt.Errorf("the microphone helper did not run: %w", err)
	}
	envelope, parseErr := swifthelper.ParseEnvelope(result.Stdout)
	if parseErr != nil {
		return errors.New("the microphone helper returned an unreadable response")
	}
	if envelope.Code != swifthelper.CodeOK {
		return fmt.Errorf("the microphone helper answered %q: %s", envelope.Code, envelope.Message)
	}
	if len(envelope.Data) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Data, dst)
}

// macStreamError maps the helper's stderr when the stream ended without samples.
func macStreamError(message string) error {
	switch {
	case strings.HasPrefix(message, "denied:"):
		return fmt.Errorf("%w: %s", platform.ErrDenied, message)
	case strings.HasPrefix(message, "not-found:"):
		return fmt.Errorf("%w: %s", platform.ErrNotFound, message)
	}
	return fmt.Errorf("the microphone stopped: %s", message)
}
