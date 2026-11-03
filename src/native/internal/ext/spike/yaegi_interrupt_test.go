//go:build spike

package spike

import (
	"context"
	"os"
	"reflect"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/traefik/yaegi/interp"
)

// These tests deliberately leave interpreted code running when it cannot be interrupted, so each
// one must run in its own process. They are gated behind SPIKE_INTERRUPT=1, for example:
//
//	SPIKE_INTERRUPT=1 go test -tags spike -run '^TestInterruptYaegiPureLoopWatchdog$' -v -count=1
func skipUnlessInterruptRun(t *testing.T) {
	t.Helper()
	if os.Getenv("SPIKE_INTERRUPT") != "1" {
		t.Skip("set SPIKE_INTERRUPT=1 and run one interrupt test per process")
	}
}

const pureLoopSource = `package main

func Run() int {
	for {
	}
}
`

const hostCallLoopSource = `package main

import "nvm"

func Run() int {
	for {
		nvm.Tick()
	}
}
`

// loadRunner evaluates source and returns its Run function and the interpreter that owns it.
func loadRunner(source string) (reflect.Value, *interp.Interpreter, error) {
	i, err := newInterpreter()
	if err != nil {
		return reflect.Value{}, nil, err
	}
	if _, err := i.Eval(source); err != nil {
		return reflect.Value{}, nil, err
	}
	run, err := i.Eval("Run")
	return run, i, err
}

// processCPU returns user plus system CPU time consumed by this process.
func processCPU() time.Duration {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0
	}
	return time.Duration(usage.Utime.Nano() + usage.Stime.Nano())
}

// watchdogCall runs fn on a goroutine and returns once the budget expires or fn finishes.
func watchdogCall(fn reflect.Value, budget time.Duration) (waited time.Duration, finished bool) {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	done := make(chan struct{})
	start := time.Now()
	go func() {
		defer close(done)
		fn.Call(nil)
	}()
	select {
	case <-ctx.Done():
	case <-done:
		finished = true
	}
	return time.Since(start), finished
}

// TestInterruptYaegiPureLoopWatchdog: a host watchdog returns on time, but the interpreted pure
// loop keeps the CPU busy afterwards. CPU time is the evidence that it was not stopped.
func TestInterruptYaegiPureLoopWatchdog(t *testing.T) {
	skipUnlessInterruptRun(t)
	fn, owner, err := loadRunner(pureLoopSource)
	requireNoError(t, err)
	waited, finished := watchdogCall(fn, 200*time.Millisecond)
	cpuBefore := processCPU()
	time.Sleep(500 * time.Millisecond)
	cpuAfter := processCPU()
	measureLine(t, "interrupt yaegi watchdog pure-loop host_returned_ms=%.1f finished=%v cpu_ms_in_500ms_after_return=%.0f (about 500 means still spinning)",
		float64(waited.Microseconds())/1e3, finished, float64((cpuAfter - cpuBefore).Milliseconds()))
	runtime.KeepAlive(owner)
}

// TestInterruptYaegiPureLoopEvalWithContext: EvalWithContext on a pure loop with no calls in it.
func TestInterruptYaegiPureLoopEvalWithContext(t *testing.T) {
	skipUnlessInterruptRun(t)
	_, owner, err := loadRunner(pureLoopSource)
	requireNoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, evalErr := owner.EvalWithContext(ctx, "Run()")
	returned := time.Since(start)
	cpuBefore := processCPU()
	time.Sleep(500 * time.Millisecond)
	cpuAfter := processCPU()
	measureLine(t, "interrupt yaegi EvalWithContext pure-loop returned_ms=%.1f err=%q cpu_ms_in_500ms_after_return=%.0f",
		float64(returned.Microseconds())/1e3, evalErr, float64((cpuAfter - cpuBefore).Milliseconds()))
	runtime.KeepAlive(owner)
}

// TestInterruptYaegiHostCallLoopEvalWithContext: the same loop with a host call in its body. The
// call boundary is where the interpreter can observe the cancellation.
func TestInterruptYaegiHostCallLoopEvalWithContext(t *testing.T) {
	skipUnlessInterruptRun(t)
	_, owner, err := loadRunner(hostCallLoopSource)
	requireNoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	before := tickCount.Load()
	start := time.Now()
	_, evalErr := owner.EvalWithContext(ctx, "Run()")
	returned := time.Since(start)
	atReturn := tickCount.Load()
	time.Sleep(500 * time.Millisecond)
	after := tickCount.Load()
	measureLine(t, "interrupt yaegi EvalWithContext host-call-loop returned_ms=%.1f err=%q ticks_at_return=%d ticks_500ms_later=%d",
		float64(returned.Microseconds())/1e3, evalErr, atReturn-before, after-atReturn)
	runtime.KeepAlive(owner)
}

// TestInterruptYaegiBlockedReceiveEvalWithContext: EvalWithContext cancels a blocked channel
// receive, the one interruptible shape measured so far.
func TestInterruptYaegiBlockedReceiveEvalWithContext(t *testing.T) {
	skipUnlessInterruptRun(t)
	source := `package main

func Run() int {
	x := <-make(chan int)
	return x
}
`
	_, owner, err := loadRunner(source)
	requireNoError(t, err)
	baseline := runtime.NumGoroutine()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, evalErr := owner.EvalWithContext(ctx, "Run()")
	returned := time.Since(start)
	time.Sleep(200 * time.Millisecond)
	measureLine(t, "interrupt yaegi EvalWithContext blocked-receive returned_ms=%.1f err=%q goroutines_before=%d goroutines_200ms_later=%d",
		float64(returned.Microseconds())/1e3, evalErr, baseline, runtime.NumGoroutine())
	runtime.KeepAlive(owner)
}

// TestInterruptYaegiBusyLoopTenSeconds runs a ten second wall-clock busy loop under a 200 ms
// watchdog. The host gives up at the budget; the loop ends only after ten seconds.
func TestInterruptYaegiBusyLoopTenSeconds(t *testing.T) {
	skipUnlessInterruptRun(t)
	source := `package main

import "nvm"

func Run() int {
	start := nvm.NowNano()
	for nvm.NowNano()-start < 10000000000 {
	}
	return 1
}
`
	fn, owner, err := loadRunner(source)
	requireNoError(t, err)
	start := time.Now()
	ended := make(chan time.Time, 1)
	go func() {
		fn.Call(nil)
		ended <- time.Now()
	}()
	time.Sleep(200 * time.Millisecond)
	hostReturned := time.Since(start)
	loopEnded := (<-ended).Sub(start)
	measureLine(t, "interrupt yaegi busy-loop-10s host_gave_up_ms=%.0f loop_ended_ms=%.0f (the host never stopped it)",
		float64(hostReturned.Milliseconds()), float64(loopEnded.Milliseconds()))
	runtime.KeepAlive(owner)
}

// TestInterruptYaegiBusyLoopTenSecondsEvalWithContext is the busy loop routed through
// EvalWithContext. If the context-aware path stops pure loops, the loop ends at the budget.
func TestInterruptYaegiBusyLoopTenSecondsEvalWithContext(t *testing.T) {
	skipUnlessInterruptRun(t)
	source := `package main

import "nvm"

func Run() int {
	start := nvm.NowNano()
	for nvm.NowNano()-start < 10000000000 {
	}
	return 1
}
`
	_, owner, err := loadRunner(source)
	requireNoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, evalErr := owner.EvalWithContext(ctx, "Run()")
	measureLine(t, "interrupt yaegi EvalWithContext busy-loop-10s returned_ms=%.0f err=%q",
		float64(time.Since(start).Milliseconds()), evalErr)
	time.Sleep(300 * time.Millisecond)
	runtime.KeepAlive(owner)
}

// TestYaegiMemoryBombIsNotCapped allocates 64 MiB inside an interpreted loop. There is no
// per-extension memory limit; only the whole process can be limited.
func TestYaegiMemoryBombIsNotCapped(t *testing.T) {
	skipUnlessInterruptRun(t)
	source := `package main

func Run() int {
	var keep [][]byte
	for i := 0; i < 64; i++ {
		keep = append(keep, make([]byte, 1<<20))
	}
	return len(keep)
}
`
	fn, owner, err := loadRunner(source)
	requireNoError(t, err)
	before := totalAllocated()
	start := time.Now()
	out := fn.Call(nil)
	elapsed := time.Since(start)
	allocated := totalAllocated() - before
	measureLine(t, "memory yaegi bomb=64MiB returned=%v elapsed_ms=%.0f allocated_mib=%.1f cap=none",
		out[0].Int(), float64(elapsed.Milliseconds()), float64(allocated)/1024/1024)
	runtime.KeepAlive(owner)
}
