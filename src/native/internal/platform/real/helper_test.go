package real

import (
	"bytes"
	"context"
	"runtime"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func requireUnixShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("helper tests run /bin/sh and are verified on Linux only")
	}
}

func TestHelperRunReturnsOutputAndExitCode(t *testing.T) {
	requireUnixShell(t)
	result, err := helper{}.Run(context.Background(), "/bin/sh", []string{"-c", "printf hi; printf oops >&2; exit 3"}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if string(result.Stdout) != "hi" || string(result.Stderr) != "oops" || result.ExitCode != 3 {
		t.Fatalf("got stdout %q stderr %q exit %d", result.Stdout, result.Stderr, result.ExitCode)
	}
}

func TestHelperRunFeedsStdin(t *testing.T) {
	requireUnixShell(t)
	result, err := helper{}.Run(context.Background(), "/bin/sh", []string{"-c", "cat"}, []byte("from stdin"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if string(result.Stdout) != "from stdin" {
		t.Fatalf("got stdout %q", result.Stdout)
	}
}

func TestHelperRunBoundsOversizedOutput(t *testing.T) {
	requireUnixShell(t)
	result, err := helper{}.Run(context.Background(), "/bin/sh", []string{"-c", "head -c 1100000 /dev/zero"}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Truncated || len(result.Stdout) != platform.MaxHelperOutputBytes {
		t.Fatalf("got %d bytes truncated=%v, want %d bytes truncated", len(result.Stdout), result.Truncated, platform.MaxHelperOutputBytes)
	}
}

func TestHelperRunReportsCancelledContextWithoutStarting(t *testing.T) {
	requireUnixShell(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := helper{}.Run(ctx, "/bin/sh", []string{"-c", "echo should not run"}, nil)
	if err != context.Canceled {
		t.Fatalf("got error %v, want context.Canceled", err)
	}
}

func TestLimitedBufferKeepsPrefixAndAcceptsEveryWrite(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		writes        []string
		wantContent   string
		wantTruncated bool
	}{
		{name: "under the limit", limit: 10, writes: []string{"ab", "cd"}, wantContent: "abcd"},
		{name: "one write crosses the limit", limit: 3, writes: []string{"abcdef"}, wantContent: "abc", wantTruncated: true},
		{name: "writes after the limit are dropped", limit: 2, writes: []string{"ab", "cd"}, wantContent: "ab", wantTruncated: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := &limitedBuffer{limit: tt.limit}
			for _, write := range tt.writes {
				n, err := buffer.Write([]byte(write))
				if err != nil || n != len(write) {
					t.Fatalf("Write(%q) = %d, %v; want full acceptance", write, n, err)
				}
			}
			if !bytes.Equal(buffer.buf.Bytes(), []byte(tt.wantContent)) || buffer.truncated != tt.wantTruncated {
				t.Fatalf("got %q truncated=%v, want %q truncated=%v", buffer.buf.Bytes(), buffer.truncated, tt.wantContent, tt.wantTruncated)
			}
		})
	}
}
