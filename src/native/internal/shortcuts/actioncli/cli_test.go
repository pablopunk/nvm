package actioncli

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// pipeServer answers each dial with an in-memory connection served by dispatch.
func pipeServer(dispatch Dispatcher) Dial {
	return func(time.Duration) (net.Conn, error) {
		client, server := net.Pipe()
		go serveOne(context.Background(), server, dispatch)
		return client, nil
	}
}

func refuse(time.Duration) (net.Conn, error) { return nil, errors.New("connection refused") }

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time       { return c.now }
func (c *fakeClock) Wait(d time.Duration) { c.now = c.now.Add(d) }

func newDeps(dial Dial) (Deps, *bytes.Buffer, *bytes.Buffer, *int) {
	var out, errOut bytes.Buffer
	starts := 0
	clock := &fakeClock{now: time.Unix(0, 0)}
	return Deps{
		Program: "nevermind-next", Dial: dial,
		Start: func() error { starts++; return nil },
		Now:   clock.Now, Wait: clock.Wait, Stdout: &out, Stderr: &errOut,
	}, &out, &errOut, &starts
}

func TestIsCommand(t *testing.T) {
	for args, want := range map[string]bool{"run x": true, "help": true, "--help": true, "-h": true, "": false, "nvm://auth": false, "--build-flags": false, "--background": false} {
		if got := IsCommand(strings.Fields(args)); got != want {
			t.Errorf("IsCommand(%q) = %v", args, got)
		}
	}
}

func TestHelpAndUsage(t *testing.T) {
	deps, out, errOut, _ := newDeps(refuse)
	if code := Main([]string{"--help"}, deps); code != ExitOK || !strings.Contains(out.String(), "nevermind-next run <action-id>") {
		t.Fatalf("help: %d %q", code, out)
	}
	if code := Main([]string{"run"}, deps); code != ExitUsage || errOut.Len() == 0 {
		t.Fatalf("missing id: %d", code)
	}
	if code := Main([]string{"run", "a", "b"}, deps); code != ExitUsage {
		t.Fatalf("extra args: %d", code)
	}
	if code := Main([]string{"run", "bad\nid"}, deps); code != ExitUsage {
		t.Fatalf("control char: %d", code)
	}
}

func TestOutcomesMapToExitStatuses(t *testing.T) {
	cases := map[Outcome]int{Completed: ExitOK, Started: ExitOK, Opened: ExitOK, Rejected: ExitRejected, Failed: ExitFailed, "weird": ExitFailed}
	for outcome, want := range cases {
		deps, _, _, starts := newDeps(pipeServer(func(context.Context, string) Response { return Response{Outcome: outcome} }))
		if code := Main([]string{"run", "command:x"}, deps); code != want {
			t.Errorf("%s: exit %d, want %d", outcome, code, want)
		}
		if *starts != 0 {
			t.Errorf("%s: started an instance while one runs", outcome)
		}
	}
}

func TestWarmRunDispatchesTheIDOnce(t *testing.T) {
	var got []string
	deps, _, _, _ := newDeps(pipeServer(func(_ context.Context, id string) Response {
		got = append(got, id)
		return Response{Outcome: Completed}
	}))
	Main([]string{"run", "extension:notes:new"}, deps)
	if len(got) != 1 || got[0] != "extension:notes:new" {
		t.Fatalf("dispatched %q", got)
	}
}

func TestColdStartStartsOnceAndDispatchesOnceReady(t *testing.T) {
	var dials, dispatches atomic.Int32
	ready := pipeServer(func(context.Context, string) Response { dispatches.Add(1); return Response{Outcome: Opened} })
	deps, _, _, starts := newDeps(func(d time.Duration) (net.Conn, error) {
		if dials.Add(1) < 5 {
			return refuse(d)
		}
		return ready(d)
	})
	if code := Main([]string{"run", "palette"}, deps); code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	if *starts != 1 || dispatches.Load() != 1 {
		t.Fatalf("starts %d dispatches %d", *starts, dispatches.Load())
	}
}

func TestColdStartTimesOutTruthfully(t *testing.T) {
	deps, _, errOut, starts := newDeps(refuse)
	if code := Main([]string{"run", "palette"}, deps); code != ExitUnavailable {
		t.Fatalf("exit %d", code)
	}
	if *starts != 1 || !strings.Contains(errOut.String(), "did not answer") {
		t.Fatalf("starts %d, %q", *starts, errOut)
	}
}

func TestStartFailureIsReported(t *testing.T) {
	deps, _, errOut, _ := newDeps(refuse)
	deps.Start = func() error { return errors.New("no such file") }
	if code := Main([]string{"run", "palette"}, deps); code != ExitUnavailable || !strings.Contains(errOut.String(), "could not start") {
		t.Fatalf("exit %d %q", code, errOut)
	}
}

func TestLostAnswerIsNeverResent(t *testing.T) {
	dials := 0
	deps, _, errOut, starts := newDeps(func(time.Duration) (net.Conn, error) {
		dials++
		client, server := net.Pipe()
		go func() {
			buf := make([]byte, 64)
			server.Read(buf)
			server.Close()
		}()
		return client, nil
	})
	if code := Main([]string{"run", "command:x"}, deps); code != ExitUnavailable {
		t.Fatalf("exit %d", code)
	}
	if dials != 1 || *starts != 0 || !strings.Contains(errOut.String(), "result was not received") {
		t.Fatalf("dials %d starts %d %q", dials, *starts, errOut)
	}
}
