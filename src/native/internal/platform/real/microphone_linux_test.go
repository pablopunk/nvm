//go:build linux

package real

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// fakePulse puts a pactl and a parec on PATH that answer like PulseAudio: pactl lists two sources (one a monitor) and
// the default; parec prints the PCM file it is given.
func fakePulse(t *testing.T, pcm []byte) {
	t.Helper()
	dir := t.TempDir()
	pcmPath := filepath.Join(dir, "pcm.raw")
	if err := os.WriteFile(pcmPath, pcm, 0o600); err != nil {
		t.Fatalf("write pcm: %v", err)
	}
	scripts := map[string]string{
		"pactl": `#!/bin/sh
case "$*" in
  *"list sources"*) echo '[{"name":"alsa_input.usb","description":"USB Mic","monitor_of_sink":null},{"name":"alsa_output.monitor","description":"Monitor","monitor_of_sink":"alsa_output"}]' ;;
  *info*) echo '{"default_source_name":"alsa_input.usb"}' ;;
esac
`,
		"parec": "#!/bin/sh\ncat '" + pcmPath + "'\n",
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o700); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestDevicesListMicrophonesAndMarkTheDefault(t *testing.T) {
	fakePulse(t, nil)
	devices, err := pulseMicrophone{}.Devices()
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	want := []platform.MicrophoneDevice{{ID: "alsa_input.usb", Title: "USB Mic", Default: true}}
	if len(devices) != len(want) || devices[0] != want[0] {
		t.Fatalf("devices = %+v, want %+v: monitors are not microphones", devices, want)
	}
}

func TestOpenStreamsSignedSixteenBitSamplesAndEndsWithEOF(t *testing.T) {
	samples := []int16{0, 1, -1, 32767, -32768, 1234}
	raw := make([]byte, 2*len(samples))
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(raw[2*i:], uint16(sample))
	}
	fakePulse(t, raw)
	stream, err := pulseMicrophone{}.Open(context.Background(), "", platform.AudioFormat{SampleRate: 16000, Channels: 1})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = stream.Close() }()
	var got []int16
	buf := make([]int16, 4)
	for {
		n, err := stream.Read(buf)
		got = append(got, buf[:n]...)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
	}
	if len(got) != len(samples) {
		t.Fatalf("read %d samples, want %d", len(got), len(samples))
	}
	for i := range samples {
		if got[i] != samples[i] {
			t.Fatalf("sample %d = %d, want %d", i, got[i], samples[i])
		}
	}
}

func TestMissingToolsAreUnsupportedNotFatal(t *testing.T) {
	if err := pulseError("parec", os.ErrNotExist); errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("a non-missing-tool error reported as unsupported")
	}
	if err := pulseError("parec", exec.ErrNotFound); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("missing parec = %v, want ErrUnsupported", err)
	}
}
