//go:build linux

package real

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	return &parecStream{cmd: cmd, out: bufio.NewReaderSize(stdout, 1<<16), stderr: &stderr}, nil
}

// parecStream reads the raw PCM parec writes. An odd byte left over from one read waits for the next.
type parecStream struct {
	cmd    *exec.Cmd
	out    *bufio.Reader
	stderr *bytes.Buffer
	carry  []byte
}

// Read fills pcm with samples, blocking until parec writes some. When parec exits, the stream ends with io.EOF,
// or with the error parec printed when it failed.
func (s *parecStream) Read(pcm []int16) (int, error) {
	want := 2 * len(pcm)
	buf := make([]byte, want)
	copy(buf, s.carry)
	n, err := io.ReadAtLeast(s.out, buf[len(s.carry):], max(0, want-len(s.carry)))
	total := len(s.carry) + n
	s.carry = nil
	if err != nil && total == 0 {
		return 0, s.endError(err)
	}
	samples := total / 2
	for i := range samples {
		pcm[i] = int16(binary.LittleEndian.Uint16(buf[2*i:]))
	}
	s.carry = append([]byte(nil), buf[2*samples:total]...)
	return samples, nil
}

func (s *parecStream) endError(readErr error) error {
	if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
		_ = s.cmd.Wait()
		if message := strings.TrimSpace(s.stderr.String()); message != "" {
			return fmt.Errorf("%w: %s", platform.ErrUnsupported, message)
		}
		return io.EOF
	}
	return readErr
}

// Close kills parec and waits for it, which unblocks a pending Read.
func (s *parecStream) Close() error {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_ = s.cmd.Wait()
	return nil
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
