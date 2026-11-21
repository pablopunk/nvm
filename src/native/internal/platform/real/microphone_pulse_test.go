//go:build linux

package real

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// TestPulseVirtualMicrophoneCarriesTheFedToneThroughOpus is the capture check for the rig: it loads a PipeWire-style
// pipe source on a running PulseAudio server, writes a 440 Hz tone into it, records the tone through parec and
// libopus, and decodes the packets back. It runs only when NVM_PULSE_TONE_TEST=1 is set, on a box or rig with a
// PulseAudio server reachable through PULSE_SERVER, and it unloads its module afterwards.
func TestPulseVirtualMicrophoneCarriesTheFedToneThroughOpus(t *testing.T) {
	if os.Getenv("NVM_PULSE_TONE_TEST") != "1" {
		t.Skip("set NVM_PULSE_TONE_TEST=1 with a PulseAudio server to run the virtual microphone check")
	}
	fifo := filepath.Join(t.TempDir(), "mic.fifo")
	module := loadPipeSource(t, fifo)
	defer unloadModule(t, module)

	const seconds = 3
	go feedTone(t, fifo, seconds)

	stream, err := pulseMicrophone{}.Open(context.Background(), "nvm_tone", platform.AudioFormat{SampleRate: 16000, Channels: 1})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = stream.Close() }()
	samples := readSamples(t, stream, seconds*16000)
	packets := encodeFrames(t, samples)
	decoded := decodeOpus(t, packets, 16000, 1)
	if rms := rootMeanSquare(decoded[len(decoded)/4:]); rms < 4000 || rms > 13000 {
		t.Fatalf("RMS of the decoded tone = %.0f; a 0.5-amplitude 440 Hz tone is about 11600", rms)
	}
	t.Logf("captured %d samples, encoded %d Opus packets, decoded RMS checked", len(samples), len(packets))
}

func loadPipeSource(t *testing.T, fifo string) string {
	t.Helper()
	out, err := exec.Command("pactl", "load-module", "module-pipe-source", "source_name=nvm_tone", "file="+fifo, "format=s16le", "rate=16000", "channels=1").CombinedOutput()
	if err != nil {
		t.Fatalf("load module-pipe-source: %v: %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func unloadModule(t *testing.T, module string) {
	t.Helper()
	if _, err := strconv.Atoi(module); err != nil {
		return
	}
	if err := exec.Command("pactl", "unload-module", module).Run(); err != nil {
		t.Errorf("unload-module %s: %v", module, err)
	}
}

// feedTone writes the tone into the pipe as fast as the server reads it, so the stream carries real-time audio.
func feedTone(t *testing.T, fifo string, seconds int) {
	pipe, err := os.OpenFile(fifo, os.O_WRONLY, 0)
	if err != nil {
		t.Errorf("open fifo: %v", err)
		return
	}
	defer func() { _ = pipe.Close() }()
	total := seconds * 16000
	raw := make([]byte, 2*320)
	for frame := 0; frame < total/320; frame++ {
		for i := 0; i < 320; i++ {
			sample := int16(0.5 * math.MaxInt16 * math.Sin(2*math.Pi*440*float64(frame*320+i)/16000))
			binary.LittleEndian.PutUint16(raw[2*i:], uint16(sample))
		}
		if _, err := pipe.Write(raw); err != nil {
			return
		}
	}
}

func readSamples(t *testing.T, stream platform.MicrophoneStream, want int) []int16 {
	t.Helper()
	var out []int16
	buf := make([]int16, 1600)
	deadline := time.Now().Add(20 * time.Second)
	for len(out) < want && time.Now().Before(deadline) {
		n, err := stream.Read(buf)
		out = append(out, buf[:n]...)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
	}
	if len(out) < want {
		t.Fatalf("captured %d samples, want %d", len(out), want)
	}
	return out[:want]
}

// encodeFrames encodes whole 20 ms frames of samples, as the dictation capture does.
func encodeFrames(t *testing.T, samples []int16) [][]byte {
	t.Helper()
	encoder, err := opusCodec{}.NewEncoder(platform.AudioFormat{SampleRate: 16000, Channels: 1})
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	defer func() { _ = encoder.Close() }()
	var packets [][]byte
	for start := 0; start+320 <= len(samples); start += 320 {
		packet, err := encoder.Encode(samples[start : start+320])
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		packets = append(packets, packet)
	}
	return packets
}

func rootMeanSquare(samples []int16) float64 {
	var sum float64
	for _, sample := range samples {
		sum += float64(sample) * float64(sample)
	}
	return math.Sqrt(sum / float64(len(samples)))
}
