// Command dictbench measures dictation latency without anyone speaking. It plays WAV files through a stand-in
// microphone at real-time speed into the real dictation service (Opus, segmenting, backend client), releases the
// dictation a moment after the clip ends, and reports how long the text took to arrive after the release, with the
// timing of every backend request. Runs are billed like real dictations.
//
//	dictbench -token-file ~/token -runs 3 speech.wav
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/dictation"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/parakeet"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

func main() {
	if code, handled := parakeet.Main(os.Args, os.Stdin, os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type options struct {
	tokenFile string
	baseURL   string
	version   string
	parakeet  string
	models    string
	recording string
	dumpWAV   string
	ortLib    string
	runs      int
	gap       time.Duration
	release   time.Duration
	clips     []string
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	flags := flag.NewFlagSet("dictbench", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var opts options
	flags.StringVar(&opts.tokenFile, "token-file", "", "file holding a Nevermind API token (required)")
	flags.StringVar(&opts.baseURL, "base-url", "", "backend origin; empty is production")
	flags.StringVar(&opts.version, "client-version", "2026.1121.1", "desktop version the backend sees; it refuses versions it no longer supports")
	flags.StringVar(&opts.parakeet, "parakeet", "", "folder of the on-device model; with -lib, transcribe on the device instead of the backend")
	flags.StringVar(&opts.models, "device", "", "folder holding the model folder; with -lib, dictate through the service on the device")
	flags.StringVar(&opts.recording, "recording", "", "with -device and -lib, replay the saved dictation with this id from the folder given as the clip")
	flags.StringVar(&opts.dumpWAV, "dump-wav", "", "with -recording, write the saved dictation's audio to this WAV file instead")
	flags.StringVar(&opts.ortLib, "lib", "", "transcribe.cpp library (libtranscribe) for -parakeet and -device")
	flags.IntVar(&opts.runs, "runs", 1, "dictations to measure")
	flags.DurationVar(&opts.gap, "gap", 800*time.Millisecond, "silence between clips")
	flags.DurationVar(&opts.release, "release", 300*time.Millisecond, "silence after the speech before the release")
	if err := flags.Parse(args); err != nil {
		return opts, err
	}
	opts.clips = flags.Args()
	if opts.tokenFile == "" && opts.parakeet == "" && opts.models == "" && opts.dumpWAV == "" || len(opts.clips) == 0 {
		return opts, errors.New("usage: dictbench (-token-file FILE | -parakeet DIR -lib LIB | -device ROOT -lib LIB) [-runs N] CLIP.wav [CLIP.wav]")
	}
	return opts, nil
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseOptions(args, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if opts.recording != "" && opts.dumpWAV != "" {
		if err := dumpRecording(opts.clips[0], opts.recording, opts.dumpWAV, stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if opts.recording != "" {
		if err := replayRecording(opts.ortLib, opts.models, opts.clips[0], opts.recording, stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	speech, err := loadSpeech(opts.clips, opts.gap)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if opts.models != "" {
		if err := runDevice(opts.ortLib, opts.models, speech, opts.runs, opts.release, stdout, stderr); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if opts.parakeet != "" {
		if err := runLocal(opts.ortLib, opts.parakeet, speech, stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	client, err := newClient(opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "clip: %.1f s of audio, %d runs\n", seconds(len(speech)), opts.runs)
	var results []runResult
	for i := range opts.runs {
		result := measure(client, speech, opts.release, stderr)
		results = append(results, result)
		printRun(stdout, i+1, result)
	}
	printSummary(stdout, results)
	return 0
}

func newClient(opts options) (*backend.Client, error) {
	raw, err := os.ReadFile(opts.tokenFile)
	if err != nil {
		return nil, fmt.Errorf("read the token: %w", err)
	}
	ports := real.NewPorts()
	logDir, err := os.MkdirTemp("", "dictbench-logs")
	if err != nil {
		return nil, err
	}
	logger, err := logx.New(ports.FS, ports.Clock, logDir, logx.Options{})
	if err != nil {
		return nil, err
	}
	env := backend.Environment{Kind: backend.EnvironmentProduction, BaseURL: opts.baseURL}
	return backend.New(ports.HTTP, logger, env, staticToken(strings.TrimSpace(string(raw))), backend.WithClock(ports.Clock), backend.WithClientVersion(opts.version))
}

type staticToken string

func (t staticToken) Get() (string, error) { return string(t), nil }

// runResult is one measured dictation.
type runResult struct {
	ReleaseToTextMs int64          `json:"releaseToTextMs"`
	Text            string         `json:"text"`
	Error           string         `json:"error,omitempty"`
	Requests        []requestTrace `json:"requests"`
}

// requestTrace is one backend transcription call, its times relative to the start of the recording.
type requestTrace struct {
	OperationID string `json:"operationId"`
	Bytes       int    `json:"bytes"`
	StartMs     int64  `json:"startMs"`
	DurationMs  int64  `json:"durationMs"`
	Error       string `json:"error,omitempty"`
}

func measure(client *backend.Client, speech []int16, release time.Duration, stderr io.Writer) runResult {
	tracer := &tracingTranscriber{client: client}
	service := dictation.NewService(dictation.Config{
		Microphone:  &replayMicrophone{speech: speech},
		Codec:       real.NewPorts().Opus,
		Transcriber: tracer,
		Warn: func(event string, fields map[string]any) {
			fmt.Fprintln(stderr, "warn:", event, fields)
		},
	})
	tracer.started = time.Now()
	if err := service.Start(""); err != nil {
		return runResult{Error: "start: " + err.Error()}
	}
	time.Sleep(time.Duration(seconds(len(speech))*float64(time.Second)) + release)
	released := time.Now()
	text, err := service.Stop()
	result := runResult{ReleaseToTextMs: time.Since(released).Milliseconds(), Text: text, Requests: tracer.traces()}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

// tracingTranscriber sends through the backend client and records every call.
type tracingTranscriber struct {
	client  *backend.Client
	started time.Time
	mu      sync.Mutex
	calls   []requestTrace
}

func (t *tracingTranscriber) Transcribe(ctx context.Context, segment dictation.Segment) (string, error) {
	begin := time.Now()
	text, err := t.client.Transcribe(ctx, backend.TranscriptionRequest{
		OperationID:    segment.OperationID,
		CancellationID: segment.CancellationID,
		Prepared:       segment.Prepared,
		Audio:          segment.File,
	})
	trace := requestTrace{
		OperationID: segment.OperationID,
		Bytes:       len(segment.File),
		StartMs:     begin.Sub(t.started).Milliseconds(),
		DurationMs:  time.Since(begin).Milliseconds(),
	}
	if err != nil {
		trace.Error = err.Error()
	}
	t.mu.Lock()
	t.calls = append(t.calls, trace)
	t.mu.Unlock()
	return text, err
}

func (t *tracingTranscriber) Prepare(ctx context.Context, operationID string) error {
	return t.client.PrepareTranscription(ctx, operationID)
}

func (t *tracingTranscriber) CancelPreparation(ctx context.Context, operationID string, userCancelled bool) error {
	return t.client.CancelTranscriptionPreparation(ctx, operationID, userCancelled)
}

func (t *tracingTranscriber) Finished(string) {}

func (t *tracingTranscriber) traces() []requestTrace {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]requestTrace(nil), t.calls...)
}

func printRun(stdout io.Writer, index int, result runResult) {
	fmt.Fprintf(stdout, "\nrun %d: release → text %d ms\n", index, result.ReleaseToTextMs)
	for _, trace := range result.Requests {
		fmt.Fprintf(stdout, "  request %-48s %7d B  start %6d ms  took %5d ms %s\n",
			trace.OperationID, trace.Bytes, trace.StartMs, trace.DurationMs, trace.Error)
	}
	if result.Error != "" {
		fmt.Fprintf(stdout, "  error: %s\n", result.Error)
	}
	fmt.Fprintf(stdout, "  text: %s\n", result.Text)
}

func printSummary(stdout io.Writer, results []runResult) {
	latencies := make([]int64, 0, len(results))
	for _, result := range results {
		if result.Error == "" {
			latencies = append(latencies, result.ReleaseToTextMs)
		}
	}
	summary, _ := json.Marshal(map[string]any{"releaseToTextMs": latencies})
	fmt.Fprintf(stdout, "\nsummary %s\n", summary)
}
