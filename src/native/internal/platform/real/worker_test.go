package real

import (
	"io"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestWorkerExchangesLinesAndReportsExitCode(t *testing.T) {
	requireUnixShell(t)
	proc, err := NewWorker().Start(platform.WorkerSpec{
		Binary: "/bin/sh",
		Args:   []string{"-c", "read line; echo \"got:$line\"; echo oops >&2; exit 3"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := proc.WriteLine([]byte("hi")); err != nil {
		t.Fatalf("WriteLine: %v", err)
	}
	line, err := proc.ReadLine()
	if err != nil || string(line) != "got:hi" {
		t.Fatalf("ReadLine = %q, %v; want got:hi", line, err)
	}
	if _, err := proc.ReadLine(); err != io.EOF {
		t.Fatalf("ReadLine after exit = %v; want io.EOF", err)
	}
	code, err := proc.Wait()
	if code != 3 {
		t.Fatalf("Wait code = %d (err %v); want 3", code, err)
	}
	if !strings.Contains(proc.StderrTail(), "oops") {
		t.Fatalf("StderrTail = %q; want oops", proc.StderrTail())
	}
}

func TestWorkerKillUnblocksReadAndIsRepeatable(t *testing.T) {
	requireUnixShell(t)
	proc, err := NewWorker().Start(platform.WorkerSpec{Binary: "/bin/sh", Args: []string{"-c", "sleep 30"}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := proc.Kill(); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if _, err := proc.ReadLine(); err == nil {
		t.Fatal("ReadLine after Kill returned no error")
	}
	if _, err := proc.Wait(); err == nil {
		t.Fatal("Wait after Kill reported a clean exit")
	}
	if err := proc.Kill(); err != nil {
		t.Fatalf("second Kill after exit = %v; want nil", err)
	}
}

func TestWorkerRejectsLinesLongerThanTheBound(t *testing.T) {
	requireUnixShell(t)
	proc, err := NewWorker().Start(platform.WorkerSpec{
		Binary:       "/bin/sh",
		Args:         []string{"-c", "printf '%0200d\\n' 0"},
		MaxLineBytes: 64,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := proc.ReadLine(); err == nil {
		t.Fatal("ReadLine accepted a line over MaxLineBytes")
	}
	_ = proc.Kill()
	_, _ = proc.Wait()
}
