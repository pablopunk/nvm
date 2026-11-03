package logx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const logDir = "/logs"

// canary is a secret-shaped value: at least forty characters with digits, upper and lower case letters,
// so the long-run rule catches it wherever it lands in free text. Tests check it never reaches the file
// or the ring, in any of the shapes the spec names.
const canary = "Qx7Canary4Lm9Zr2Tb8Wk5Vy1Ps6Nd3Hf0Jg8RcQa"

func newTestLogger(t *testing.T, opts Options) (*Logger, *fake.FS, *fake.Clock) {
	t.Helper()
	files := fake.NewFS()
	clock := fake.NewClock()
	logger, err := New(files, clock, logDir, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	return logger, files, clock
}

func activeFile(t *testing.T, files *fake.FS) []byte {
	t.Helper()
	data, err := files.ReadFile(logDir + "/nvm.log")
	if err != nil {
		t.Fatalf("read active log: %v", err)
	}
	return data
}

func decodeLines(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("line is not JSON: %q: %v", line, err)
		}
		out = append(out, entry)
	}
	return out
}

func TestEntriesAreWrittenAsJSONLinesWithFieldsAndTime(t *testing.T) {
	logger, files, _ := newTestLogger(t, Options{})
	logger.Info("sign-in finished", F{"provider": "github", "attempts": 2})
	logger.Error("save failed", F{"error": errors.New("disk full")})
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	lines := decodeLines(t, activeFile(t, files))
	if len(lines) != 2 {
		t.Fatalf("wrote %d lines, want 2", len(lines))
	}
	first := lines[0]
	if first["msg"] != "sign-in finished" || first["level"] != "info" {
		t.Fatalf("first line = %v", first)
	}
	if first["time"] != "2026-01-01T09:00:00Z" {
		t.Fatalf("time = %v, want the fake clock epoch", first["time"])
	}
	fields := first["fields"].(map[string]any)
	if fields["provider"] != "github" || fields["attempts"] != float64(2) {
		t.Fatalf("fields = %v", fields)
	}
	if lines[1]["level"] != "error" {
		t.Fatalf("second line level = %v", lines[1]["level"])
	}
}

func TestLevelFilterDropsEntriesBelowTheMinimum(t *testing.T) {
	logger, _, _ := newTestLogger(t, Options{Level: Warn})
	logger.Debug("noise", nil)
	logger.Info("also noise", nil)
	logger.Warn("kept", nil)
	entries := logger.Recent(0)
	if len(entries) != 1 || entries[0].Message != "kept" {
		t.Fatalf("entries = %+v, want only the warning", entries)
	}
}

func TestChildLoggersAddTheirFieldsOnly(t *testing.T) {
	logger, _, _ := newTestLogger(t, Options{})
	child := logger.With(F{"component": "clipboard"}).With(F{"op": "poll"})
	child.Info("tick", F{"items": 3})
	logger.Info("root", nil)

	entries := logger.Recent(0)
	if entries[0].Fields["component"] != "clipboard" || entries[0].Fields["op"] != "poll" || entries[0].Fields["items"] != 3 {
		t.Fatalf("child fields = %v", entries[0].Fields)
	}
	if _, leaked := entries[1].Fields["component"]; leaked {
		t.Fatalf("root entry inherited child fields: %v", entries[1].Fields)
	}
}

// TestCanarySecretsNeverReachTheFileOrTheRing is the redaction guard from the spec: the canary is passed
// as a keyed field, inside a message, inside an error string, inside a URL query, and nested in a header
// map. None of those copies may appear in the file or in the ring buffer.
func TestCanarySecretsNeverReachTheFileOrTheRing(t *testing.T) {
	if len(canary) < 40 {
		t.Fatalf("canary is %d characters; it must reach the long-run rule", len(canary))
	}
	logger, files, _ := newTestLogger(t, Options{})
	type request struct {
		Token string `json:"token"`
		Path  string `json:"path"`
	}
	logger.Info("retrying with token="+canary, F{"token": canary})
	logger.Warn("auth header Bearer "+canary, F{"note": "stored " + canary})
	logger.Error("dial failed", F{"error": fmt.Errorf("dial https://api.example.test/v1?token=%s: refused", canary)})
	logger.Info("callback", F{"url": "https://app.example.test/callback?code=" + canary + "&state=ok"})
	logger.Info("request", F{"headers": map[string]any{"Authorization": "Bearer " + canary}})
	logger.Info("payload", F{"request": request{Token: canary, Path: "/v1/sync"}})
	logger.Info("bare", F{"text": "api_key=" + canary})
	logger.Info("sign-in finished", F{"provider": "github"})
	if err := logger.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	file := activeFile(t, files)
	ring := logger.Recent(0)
	ringJSON, err := json.Marshal(ring)
	if err != nil {
		t.Fatal(err)
	}
	for _, surface := range []struct {
		name string
		data []byte
	}{{"file", file}, {"ring", ringJSON}} {
		if bytes.Contains(surface.data, []byte(canary)) {
			t.Errorf("canary present in %s", surface.name)
		}
		if bytes.Contains(surface.data, []byte(canary[:12])) {
			t.Errorf("canary prefix present in %s", surface.name)
		}
	}
	if !bytes.Contains(file, []byte("sign-in finished")) || !bytes.Contains(file, []byte("github")) {
		t.Fatal("redaction removed ordinary content too")
	}
}

func TestErrorEntriesReachTheFileWithoutWaitingForTheTimer(t *testing.T) {
	logger, files, _ := newTestLogger(t, Options{})
	logger.Error("boom", nil)
	if !strings.Contains(string(activeFile(t, files)), `"msg":"boom"`) {
		t.Fatal("error entry was not written at once")
	}
}

func TestIdleEntriesReachTheFileOnTheFlushTick(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	writes := make(chan string, 10)
	files.AfterWrite(func(name string) error {
		writes <- name
		return nil
	})
	logger, err := New(files, clock, logDir, Options{FlushInterval: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Close() })

	logger.Info("quiet line", nil)
	clock.AwaitTimers(1)
	clock.Advance(time.Second)
	<-writes
	// Flush takes the same lock as the flusher, so it returns only after the tick's write has finished.
	if err := logger.Flush(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(activeFile(t, files)), "quiet line") {
		t.Fatal("the tick did not write the idle entry")
	}
}

func TestCloseWritesWhatIsPending(t *testing.T) {
	logger, files, _ := newTestLogger(t, Options{})
	logger.Info("pending on close", nil)
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !strings.Contains(string(activeFile(t, files)), "pending on close") {
		t.Fatal("Close did not write the pending entry")
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestFailedWriteDropsTheBatchAndCountsIt(t *testing.T) {
	logger, files, _ := newTestLogger(t, Options{})
	files.FailNext(fake.OpWriteFile, errors.New("disk full"))
	logger.Error("lost on the way", nil)
	if got := logger.Dropped(); got != 1 {
		t.Fatalf("Dropped = %d, want 1", got)
	}
	if got := logger.Recent(0); len(got) != 1 {
		t.Fatalf("the ring kept %d entries, want the lost one visible in memory", len(got))
	}
}

func TestStderrMirrorReceivesEveryKeptEntry(t *testing.T) {
	var mirror bytes.Buffer
	logger, _, _ := newTestLogger(t, Options{Stderr: &mirror})
	logger.Info("one", nil)
	logger.Debug("hidden", nil)
	lines := decodeLines(t, mirror.Bytes())
	if len(lines) != 1 || lines[0]["msg"] != "one" {
		t.Fatalf("mirror = %v", lines)
	}
}

func TestMemoryOnlyLoggerKeepsRingAndNoPath(t *testing.T) {
	logger, err := New(nil, fake.NewClock(), "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("in memory", nil)
	if logger.Path() != "" || len(logger.Recent(0)) != 1 {
		t.Fatalf("path %q, ring %d", logger.Path(), len(logger.Recent(0)))
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestConcurrentWritersKeepEveryLineWhole runs under -race: many goroutines log through child loggers
// while flushes happen inline, and the file must still hold one valid JSON object per entry.
func TestConcurrentWritersKeepEveryLineWhole(t *testing.T) {
	logger, files, _ := newTestLogger(t, Options{})
	const writers, perWriter = 8, 100
	var wg sync.WaitGroup
	for w := range writers {
		child := logger.With(F{"writer": w})
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWriter {
				child.Info("concurrent line", F{"i": i})
			}
		}()
	}
	wg.Wait()
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if lines := decodeLines(t, activeFile(t, files)); len(lines) != writers*perWriter {
		t.Fatalf("file holds %d lines, want %d", len(lines), writers*perWriter)
	}
}
