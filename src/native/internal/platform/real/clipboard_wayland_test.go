package real

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// feedLine frames one copy the way waylandWatchScript does.
func feedLine(state, mime string, payload []byte) string {
	return state + " " + mime + " " + base64.StdEncoding.EncodeToString(payload) + "\n"
}

// fakeWatchProcesses hands out one scripted output per start of the watch process and records the commands.
type fakeWatchProcesses struct {
	mu      sync.Mutex
	outputs []string
	starts  [][]string
	failure error
}

func (f *fakeWatchProcesses) start(_ context.Context, binary string, args ...string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts = append(f.starts, append([]string{binary}, args...))
	if f.failure != nil {
		return nil, f.failure
	}
	output := ""
	if len(f.outputs) > 0 {
		output, f.outputs = f.outputs[0], f.outputs[1:]
	}
	return io.NopCloser(strings.NewReader(output)), nil
}

func (f *fakeWatchProcesses) startCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.starts)
}

func newTestFeed(env map[string]string, tools []string, processes *fakeWatchProcesses) *waylandFeed {
	feed := newWaylandFeed("linux", environment(env), toolSet(tools...), processes.start)
	feed.restartDelay = 0
	return feed
}

func collect(t *testing.T, copies <-chan platform.ClipboardCopy, want int) []platform.ClipboardCopy {
	t.Helper()
	var got []platform.ClipboardCopy
	timeout := time.After(5 * time.Second)
	for len(got) < want {
		select {
		case copied, open := <-copies:
			if !open {
				t.Fatalf("feed closed after %d copies, want %d", len(got), want)
			}
			got = append(got, copied)
		case <-timeout:
			t.Fatalf("timed out after %d copies, want %d", len(got), want)
		}
	}
	return got
}

// TestWaylandFeedAvailabilityMatrix covers session type against the tools the feed needs.
func TestWaylandFeedAvailabilityMatrix(t *testing.T) {
	allTools := []string{"wl-paste", "sh", "base64"}
	tests := []struct {
		name      string
		env       map[string]string
		tools     []string
		wantFeed  bool
		wantError bool
	}{
		{"Wayland with every tool", waylandEnv, allTools, true, false},
		{"Wayland without wl-paste", waylandEnv, []string{"sh", "base64"}, false, true},
		{"Wayland without base64", waylandEnv, []string{"wl-paste", "sh"}, false, true},
		{"XWayland session", xwaylandEnv, allTools, true, false},
		{"X11 with every tool", x11Env, allTools, false, false},
		{"X11 without wl-paste", x11Env, nil, false, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			processes := &fakeWatchProcesses{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			copies, err := newTestFeed(test.env, test.tools, processes).WatchCopies(ctx)
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, want error %v", err, test.wantError)
			}
			if err != nil && !errors.Is(err, platform.ErrUnsupported) {
				t.Fatalf("error = %v, want ErrUnsupported", err)
			}
			if (copies != nil) != test.wantFeed {
				t.Fatalf("channel present = %v, want %v", copies != nil, test.wantFeed)
			}
			if !test.wantFeed && processes.startCount() != 0 {
				t.Fatalf("started the watch process %d times without a feed", processes.startCount())
			}
		})
	}
}

// TestWaylandFeedDeliversEveryCopyOfABurst: each copy arrives as its own event with its content, text and
// images alike, and the first one is the baseline.
func TestWaylandFeedDeliversEveryCopyOfABurst(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nfake")
	processes := &fakeWatchProcesses{outputs: []string{
		feedLine("data", "text/plain;charset=utf-8", []byte("already there")) +
			feedLine("data", "text/plain;charset=utf-8", []byte("first")) +
			feedLine("data", "image/png", png) +
			feedLine("data", "UTF8_STRING", []byte("third")),
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	copies, err := newTestFeed(waylandEnv, []string{"wl-paste", "sh", "base64"}, processes).WatchCopies(ctx)
	if err != nil {
		t.Fatalf("WatchCopies = %v", err)
	}
	got := collect(t, copies, 4)
	if !got[0].Initial || got[0].Text != "already there" {
		t.Fatalf("first copy = %+v, want the baseline", got[0])
	}
	if got[1].Initial || got[1].Text != "first" {
		t.Fatalf("second copy = %+v", got[1])
	}
	if got[2].Initial || !slices.Equal(got[2].PNG, png) || got[2].Text != "" {
		t.Fatalf("third copy = %+v, want the PNG", got[2])
	}
	if got[3].Initial || got[3].Text != "third" {
		t.Fatalf("fourth copy = %+v", got[3])
	}
	wantStart := []string{"wl-paste", "--watch", "sh", "-c", waylandWatchScript}
	if !slices.Equal(processes.starts[0], wantStart) {
		t.Fatalf("watch command = %v, want %v", processes.starts[0], wantStart)
	}
}

func TestWaylandFeedParsesSpecialCopies(t *testing.T) {
	tests := []struct {
		name string
		line string
		want platform.ClipboardCopy
		ok   bool
	}{
		{"sensitive copy keeps no content", "sensitive - \n", platform.ClipboardCopy{Concealed: true}, true},
		{"emptied clipboard is not a copy", "nil - \n", platform.ClipboardCopy{}, false},
		{"file manager copy", feedLine("data", "text/uri-list", []byte("file:///home/a/My%20File.txt\r\nfile:///home/a/b.png\r\n")), platform.ClipboardCopy{Files: []string{"/home/a/My File.txt", "/home/a/b.png"}}, true},
		{"web links stay text", feedLine("data", "text/uri-list", []byte("https://example.com\r\n")), platform.ClipboardCopy{Text: "https://example.com\r\n"}, true},
		{"jpeg is skipped", feedLine("data", "image/jpeg", []byte("jpeg")), platform.ClipboardCopy{}, false},
		{"empty data is skipped", "data text/plain \n", platform.ClipboardCopy{}, false},
		{"broken base64 is skipped", "data text/plain !!!\n", platform.ClipboardCopy{}, false},
		{"unknown type is skipped", feedLine("data", "application/x-thing", []byte("x")), platform.ClipboardCopy{}, false},
		{"short line is skipped", "garbage\n", platform.ClipboardCopy{}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := parseWaylandCopy([]byte(strings.TrimSuffix(test.line, "\n")))
			if ok != test.ok || !slices.Equal(got.Files, test.want.Files) || got.Text != test.want.Text || got.Concealed != test.want.Concealed {
				t.Fatalf("parse = %+v, %v; want %+v, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

// TestWaylandFeedRestartsTheWatchProcessAndKeepsLaterCopies: a copy after a restart is a copy, not a baseline.
func TestWaylandFeedRestartsTheWatchProcessAndKeepsLaterCopies(t *testing.T) {
	processes := &fakeWatchProcesses{outputs: []string{
		feedLine("data", "text/plain", []byte("baseline")),
		feedLine("data", "text/plain", []byte("after restart")),
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	copies, err := newTestFeed(waylandEnv, []string{"wl-paste", "sh", "base64"}, processes).WatchCopies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := collect(t, copies, 2)
	if !got[0].Initial || got[1].Initial || got[1].Text != "after restart" {
		t.Fatalf("copies = %+v", got)
	}
}

// TestWaylandFeedGivesUpWhenTheWatchProcessKeepsExitingEmpty: a compositor without the data-control protocol makes
// wl-paste exit at once; the feed closes so the history falls back to reading the clipboard.
func TestWaylandFeedGivesUpWhenTheWatchProcessKeepsExitingEmpty(t *testing.T) {
	processes := &fakeWatchProcesses{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	copies, err := newTestFeed(waylandEnv, []string{"wl-paste", "sh", "base64"}, processes).WatchCopies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case _, open := <-copies:
		if open {
			t.Fatal("received a copy from a silent process")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("feed never closed")
	}
	if got := processes.startCount(); got != maxEmptyWatchStarts {
		t.Fatalf("starts = %d, want %d", got, maxEmptyWatchStarts)
	}
}

func TestWaylandFeedReportsAStartFailure(t *testing.T) {
	processes := &fakeWatchProcesses{failure: errors.New("exec failed")}
	copies, err := newTestFeed(waylandEnv, []string{"wl-paste", "sh", "base64"}, processes).WatchCopies(context.Background())
	if err == nil || copies != nil {
		t.Fatalf("WatchCopies = %v, %v; want an error", copies, err)
	}
}

func TestReadFeedLineSkipsOversizedCopiesAndKeepsTheNext(t *testing.T) {
	limit := 30
	big := strings.Repeat("A", 200)
	reader := bufio.NewReaderSize(strings.NewReader(big+"\nsmall\n"), 16)
	if line, err := readFeedLine(reader, limit); line != nil || !errors.Is(err, errFeedLineTooLong) {
		t.Fatalf("oversized = %q, %v", line, err)
	}
	line, err := readFeedLine(reader, limit)
	if err != nil || string(line) != "small" {
		t.Fatalf("next line = %q, %v", line, err)
	}
	if _, err := readFeedLine(reader, limit); !errors.Is(err, io.EOF) {
		t.Fatalf("end = %v, want EOF", err)
	}
}

// TestWaylandWatchScriptIsPlainPOSIXShell keeps the script free of bashisms: it runs under /bin/sh.
func TestWaylandWatchScriptIsPlainPOSIXShell(t *testing.T) {
	for _, banned := range []string{"[[", "<<<", "$'", "echo -e", "local "} {
		if strings.Contains(waylandWatchScript, banned) {
			t.Fatalf("script uses %q", banned)
		}
	}
}
