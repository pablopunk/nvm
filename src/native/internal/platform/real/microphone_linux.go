//go:build linux

package real

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// pulseMicrophone records through PulseAudio's or PipeWire-Pulse's command-line tools: pactl lists the sources
// and parec streams raw PCM from one. Both ship with pulseaudio-utils, so a desktop without them has no
// microphone capture (ErrUnsupported), and a server that refuses the connection says so the same way.
type pulseMicrophone struct{}

func newMicrophone() platform.Microphone {
	return pulseMicrophone{}
}

// pactlSource is one entry of `pactl -f json list sources`. Monitors of output devices are not microphones.
type pactlSource struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	MonitorOf   *string `json:"monitor_of_sink"`
}

// Devices lists the input devices and marks the server's default source.
func (pulseMicrophone) Devices() ([]platform.MicrophoneDevice, error) {
	sources, err := pulseSources()
	if err != nil {
		return nil, err
	}
	defaultName, err := pulseDefaultSource()
	if err != nil {
		return nil, err
	}
	var devices []platform.MicrophoneDevice
	for _, source := range sources {
		if source.MonitorOf != nil || source.Name == "" {
			continue
		}
		title := source.Description
		if title == "" {
			title = source.Name
		}
		devices = append(devices, platform.MicrophoneDevice{ID: source.Name, Title: title, Default: source.Name == defaultName})
	}
	return devices, nil
}

func pulseSources() ([]pactlSource, error) {
	out, err := exec.Command("pactl", "-f", "json", "list", "sources").Output()
	if err != nil {
		return nil, pulseError("pactl list sources", err)
	}
	var sources []pactlSource
	if err := json.Unmarshal(out, &sources); err != nil {
		return nil, fmt.Errorf("pactl returned sources that are not JSON: %w", err)
	}
	return sources, nil
}

func pulseDefaultSource() (string, error) {
	out, err := exec.Command("pactl", "-f", "json", "info").Output()
	if err != nil {
		return "", pulseError("pactl info", err)
	}
	var info struct {
		DefaultSource string `json:"default_source_name"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return "", fmt.Errorf("pactl returned server info that is not JSON: %w", err)
	}
	return info.DefaultSource, nil
}

// Open starts parec on the device and returns a stream of signed 16-bit little-endian samples.
func (pulseMicrophone) Open(ctx context.Context, deviceID string, format platform.AudioFormat) (platform.MicrophoneStream, error) {
	args := []string{"--raw", "--format=s16le", fmt.Sprintf("--rate=%d", format.SampleRate), fmt.Sprintf("--channels=%d", format.Channels)}
	if deviceID != "" && deviceID != "default" {
		args = append(args, "--device="+deviceID)
	}
	cmd := exec.CommandContext(ctx, "parec", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, pulseError("parec", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, pulseError("parec", err)
	}
	return newPCMProcessStream(cmd, stdout, &stderr, pulseStreamError), nil
}

// pulseStreamError reports a parec that failed while recording: the OS cannot capture until the server recovers.
func pulseStreamError(message string) error {
	return fmt.Errorf("%w: %s", platform.ErrUnsupported, message)
}

// pulseError maps a failed tool run to a platform error: a missing tool or a refused server means the OS cannot
// capture, and anything else is reported with the tool's name.
func pulseError(tool string, err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("%w: %s is not installed", platform.ErrUnsupported, tool)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && strings.Contains(string(exitErr.Stderr), "Connection") {
		return fmt.Errorf("%w: no audio server is running", platform.ErrUnsupported)
	}
	return fmt.Errorf("%s failed: %w", tool, err)
}
