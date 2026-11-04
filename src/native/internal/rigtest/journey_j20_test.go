//go:build rig

package rigtest

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	dict "github.com/pablopunk/nvm/native/internal/dictation"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// j20Token is the bearer token the journey's client sends.
type j20Token string

func (t j20Token) Get() (string, error) { return string(t), nil }

// j20Transcriber sends dictation segments through the real backend client.
type j20Transcriber struct {
	client *backend.Client
}

func (t j20Transcriber) Prepare(ctx context.Context, operationID string) error {
	return t.client.PrepareTranscription(ctx, operationID)
}

func (t j20Transcriber) Transcribe(ctx context.Context, segment dict.Segment) (string, error) {
	return t.client.Transcribe(ctx, backend.TranscriptionRequest{
		OperationID:    segment.OperationID,
		CancellationID: segment.CancellationID,
		Prepared:       segment.Prepared,
		Audio:          segment.File,
	})
}

func (t j20Transcriber) CancelPreparation(ctx context.Context, operationID string, userCancelled bool) error {
	return t.client.CancelTranscriptionPreparation(ctx, operationID, userCancelled)
}

// TestJourneyJ20DictationPipelineCarriesToneToTranscript runs the dictation pipeline on a private PulseAudio server. A
// 440 Hz tone is written into a pipe source that stands in for the microphone; the real capture and libopus encode it;
// the real backend client sends the WebM to a fake backend; the transcript comes back. The paste into a focused app and
// the palette's Dictate command need the running app and a text target, so they are not part of this journey.
func TestJourneyJ20DictationPipelineCarriesToneToTranscript(t *testing.T) {
	if os.Getenv("PULSE_SERVER") == "" {
		t.Skip("needs a PulseAudio server reachable through PULSE_SERVER")
	}
	uploads := make(chan []byte, 4)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/audio/transcription-preparations":
			_, _ = w.Write([]byte(`{}`))
		case "/api/v1/audio/transcriptions":
			body, _ := io.ReadAll(r.Body)
			if r.Header.Get("Content-Type") != "audio/webm" {
				http.Error(w, `{"error":{"type":"invalid_request","message":"want audio/webm"}}`, http.StatusBadRequest)
				return
			}
			uploads <- body
			_, _ = w.Write([]byte(`{"text":"hello from the rig"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()

	ports := real.NewPorts()
	env, err := backend.NewEnvironment(backend.EnvironmentDevelopment, fake.URL, false)
	if err != nil {
		t.Fatalf("environment: %v", err)
	}
	logger, err := logx.New(ports.FS, ports.Clock, t.TempDir(), logx.Options{FlushInterval: time.Second, Level: logx.Info})
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	defer func() { _ = logger.Close() }()
	client, err := backend.New(ports.HTTP, logger, env, j20Token("rig-token"), backend.WithClock(ports.Clock))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	service := dict.NewService(dict.Config{Microphone: ports.Microphone, Codec: ports.Opus, Transcriber: j20Transcriber{client: client}})

	fifo := filepath.Join(t.TempDir(), "rig-mic.fifo")
	module := loadJ20Source(t, fifo)
	defer unloadJ20Module(t, module)

	if err := service.Start("nvm_rig_tone"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	feedJ20Tone(t, fifo, 2)
	text, err := service.Stop()
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if text != "hello from the rig" {
		t.Fatalf("transcript = %q, want the fake backend's text", text)
	}
	select {
	case body := <-uploads:
		if !bytes.HasPrefix(body, []byte{0x1A, 0x45, 0xDF, 0xA3}) || len(body) < 2000 {
			t.Fatalf("uploaded %d bytes, want a WebM file of speech", len(body))
		}
	default:
		t.Fatalf("the fake backend received no audio")
	}
}

// loadJ20Source loads a PulseAudio pipe source named nvm_rig_tone; PulseAudio creates the FIFO.
func loadJ20Source(t *testing.T, fifo string) string {
	t.Helper()
	out, err := exec.Command("pactl", "load-module", "module-pipe-source", "source_name=nvm_rig_tone", "file="+fifo, "format=s16le", "rate=16000", "channels=1").CombinedOutput()
	if err != nil {
		t.Fatalf("load module-pipe-source: %v: %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func unloadJ20Module(t *testing.T, module string) {
	t.Helper()
	if err := exec.Command("pactl", "unload-module", module).Run(); err != nil {
		t.Errorf("unload-module %s: %v", module, err)
	}
}

// feedJ20Tone writes seconds of tone in real time, one 20 ms frame at a time.
func feedJ20Tone(t *testing.T, fifo string, seconds int) {
	t.Helper()
	pipe, err := os.OpenFile(fifo, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open fifo: %v", err)
	}
	defer func() { _ = pipe.Close() }()
	raw := make([]byte, 2*320)
	for frame := 0; frame < seconds*50; frame++ {
		for i := 0; i < 320; i++ {
			sample := int16(0.5 * math.MaxInt16 * math.Sin(2*math.Pi*440*float64(frame*320+i)/16000))
			binary.LittleEndian.PutUint16(raw[2*i:], uint16(sample))
		}
		if _, err := pipe.Write(raw); err != nil {
			t.Fatalf("write tone: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
