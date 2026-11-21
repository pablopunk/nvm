//go:build spike && linux

package spike

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// The subprocess sketch: the host spawns a child (this test binary in child mode), writes one JSON
// request per line on stdin and reads one JSON response per line on stdout. The child holds the
// Yaegi interpreter, so a crash, hang or memory bomb in the extension cannot take the host down.

type rpcRequest struct {
	ID    int    `json:"id"`
	Query string `json:"query"`
}

type rpcResponse struct {
	ID    int    `json:"id"`
	Items []Item `json:"items,omitempty"`
	Error string `json:"error,omitempty"`
}

// childProcess is the host side of one extension child.
type childProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
	nextID int
}

// startChild spawns the child in the given extension mode and waits for its ready line.
func startChild(t *testing.T, mode string, rlimitMiB int) (*childProcess, time.Duration, error) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		return nil, 0, err
	}
	cmd := exec.Command(exe, "-test.run=^TestChildServe$")
	cmd.Env = append(os.Environ(), "SPIKE_CHILD=serve", "SPIKE_EXT="+mode, "SPIKE_RLIMIT_MIB="+strconv.Itoa(rlimitMiB))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, 0, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, 0, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}
	child := &childProcess{cmd: cmd, stdin: stdin, reader: bufio.NewReader(stdout)}
	line, err := child.reader.ReadBytes('\n')
	if err != nil {
		_ = child.kill()
		return nil, 0, fmt.Errorf("child did not become ready: %w; stderr: %s", err, firstLine(stderr.String()))
	}
	if string(line) != "{\"ready\":true}\n" {
		_ = child.kill()
		return nil, 0, fmt.Errorf("unexpected ready line %q", line)
	}
	return child, time.Since(start), nil
}

// call sends one request and reads one response line. It returns the response and the round trip.
func (c *childProcess) call(query string) (rpcResponse, time.Duration, error) {
	c.nextID++
	payload, err := json.Marshal(rpcRequest{ID: c.nextID, Query: query})
	if err != nil {
		return rpcResponse{}, 0, err
	}
	start := time.Now()
	if _, err := c.stdin.Write(append(payload, '\n')); err != nil {
		return rpcResponse{}, 0, err
	}
	line, err := c.reader.ReadBytes('\n')
	elapsed := time.Since(start)
	if err != nil {
		return rpcResponse{}, elapsed, err
	}
	var response rpcResponse
	if err := json.Unmarshal(line, &response); err != nil {
		return rpcResponse{}, elapsed, err
	}
	return response, elapsed, nil
}

// kill terminates the child and waits for it, returning how long the reap took.
func (c *childProcess) kill() error {
	if c.cmd.Process == nil {
		return nil
	}
	if err := c.cmd.Process.Kill(); err != nil {
		return err
	}
	_, _ = c.cmd.Process.Wait()
	return nil
}

// TestMeasureSubprocessIsolation reports startup, round-trip latency over 1000 calls, kill and
// restart timing, and the OS-level memory limit behaviour.
func TestMeasureSubprocessIsolation(t *testing.T) {
	child, startup, err := startChild(t, "trivial", 0)
	requireNoError(t, err)
	measureLine(t, "subprocess startup_to_ready_ms=%.1f extension=trivial", float64(startup.Microseconds())/1e3)
	samples := make([]time.Duration, 0, 1000)
	for i := range 1000 {
		response, elapsed, err := child.call("one")
		requireNoError(t, err)
		if i == 0 && len(response.Items) != 3 {
			t.Fatalf("trivial child returned %d items", len(response.Items))
		}
		samples = append(samples, elapsed)
	}
	p50, p95, p99 := summarize(samples)
	measureLine(t, "subprocess roundtrip trivial calls=1000 p50_us=%.1f p95_us=%.1f p99_us=%.1f", p50, p95, p99)
	requireNoError(t, child.kill())

	realistic, startup, err := startChild(t, "realistic", 0)
	requireNoError(t, err)
	measureLine(t, "subprocess startup_to_ready_ms=%.1f extension=realistic", float64(startup.Microseconds())/1e3)
	samples = samples[:0]
	for range 1000 {
		_, elapsed, err := realistic.call("item-0")
		requireNoError(t, err)
		samples = append(samples, elapsed)
	}
	p50, p95, p99 = summarize(samples)
	measureLine(t, "subprocess roundtrip realistic calls=1000 p50_us=%.1f p95_us=%.1f p99_us=%.1f", p50, p95, p99)
	requireNoError(t, realistic.kill())
}

// TestMeasureSubprocessKillOnTimeout simulates an extension that never returns. The host gives it a
// 200 ms budget, kills the child, and restarts a fresh child.
func TestMeasureSubprocessKillOnTimeout(t *testing.T) {
	child, _, err := startChild(t, "hang", 0)
	requireNoError(t, err)

	deadline := time.Now().Add(200 * time.Millisecond)
	responses := make(chan error, 1)
	go func() {
		_, _, err := child.call("anything")
		responses <- err
	}()
	select {
	case <-responses:
		t.Fatal("hanging extension answered")
	case <-time.After(time.Until(deadline)):
	}
	killStart := time.Now()
	requireNoError(t, child.kill())
	killTook := time.Since(killStart)
	readErr := <-responses
	measureLine(t, "subprocess kill-on-timeout budget_ms=200 kill_and_reap_ms=%.2f pending_read_error=%q",
		float64(killTook.Microseconds())/1e3, firstLine(fmt.Sprint(readErr)))

	restarted, startup, err := startChild(t, "trivial", 0)
	requireNoError(t, err)
	measureLine(t, "subprocess restart-after-kill startup_ms=%.1f", float64(startup.Microseconds())/1e3)
	requireNoError(t, restarted.kill())
}

// TestMeasureSubprocessGoroutinePanic runs an extension that panics on a goroutine it spawned.
// Recover on the host cannot catch it, so the child dies; the host only observes the exit.
func TestMeasureSubprocessGoroutinePanic(t *testing.T) {
	child, _, err := startChild(t, "goroutine-panic", 0)
	requireNoError(t, err)
	_, _, callErr := child.call("x")
	waitStart := time.Now()
	waitErr := child.cmd.Wait()
	measureLine(t, "subprocess goroutine-panic call_err=%q exit=%q exit_wait_ms=%.1f host_alive=yes",
		firstLine(fmt.Sprint(callErr)), firstLine(fmt.Sprint(waitErr)), float64(time.Since(waitStart).Microseconds())/1e3)
}

// TestMeasureSubprocessMemoryLimit gives the child an address-space limit and runs an allocation
// bomb. The OS limit ends the child; the host keeps running and sees a dead pipe.
func TestMeasureSubprocessMemoryLimit(t *testing.T) {
	var child *childProcess
	var startup time.Duration
	limitMiB := 0
	for _, candidate := range []int{512, 1024, 2048} {
		started, took, err := startChild(t, "bomb", candidate)
		if err != nil {
			measureLine(t, "subprocess memory-limit rlimit_as_mib=%d status=child-did-not-start err=%q", candidate, firstLine(err.Error()))
			continue
		}
		measureLine(t, "subprocess memory-limit rlimit_as_mib=%d status=child-started startup_ms=%.1f", candidate, float64(took.Microseconds())/1e3)
		child, startup, limitMiB = started, took, candidate
		break
	}
	if child == nil {
		t.Skip("no candidate address-space limit let the child start")
	}
	_ = startup
	start := time.Now()
	_, _, callErr := child.call("x")
	waitErr := child.cmd.Wait()
	state := "unknown"
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		state = fmt.Sprint(exitErr.ExitCode())
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			state = "signal " + status.Signal().String()
		}
	}
	measureLine(t, "subprocess memory-limit rlimit_as_mib=%d killed_after_ms=%.0f exit=%s call_err=%q",
		limitMiB, float64(time.Since(start).Milliseconds()), state, firstLine(fmt.Sprint(callErr)))
}

// TestChildServe is the child side. It runs only when SPIKE_CHILD=serve, which the parent sets.
func TestChildServe(t *testing.T) {
	if os.Getenv("SPIKE_CHILD") != "serve" {
		t.Skip("child mode only")
	}
	if mib, _ := strconv.Atoi(os.Getenv("SPIKE_RLIMIT_MIB")); mib > 0 {
		limit := uint64(mib) << 20
		requireNoError(t, syscall.Setrlimit(syscall.RLIMIT_AS, &syscall.Rlimit{Cur: limit, Max: limit}))
	}
	source := childSource(os.Getenv("SPIKE_EXT"))
	ext, err := loadYaegi(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		os.Exit(3)
	}
	fmt.Println(`{"ready":true}`)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request rpcRequest
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			fmt.Println(`{"error":"bad request"}`)
			continue
		}
		items, err := ext.Search(request.Query)
		response := rpcResponse{ID: request.ID, Items: items}
		if err != nil {
			response.Error = err.Error()
		}
		payload, _ := json.Marshal(response)
		fmt.Println(string(payload))
	}
}

func childSource(mode string) string {
	switch mode {
	case "realistic":
		return RealisticGo()
	case "hang":
		return `package main

import "nvm"

func Search(q string) []nvm.Item {
	for {
		nvm.Tick()
	}
}
`
	case "goroutine-panic":
		return `package main

import (
	"nvm"
	"time"
)

func Search(q string) []nvm.Item {
	go func() {
		time.Sleep(20 * time.Millisecond)
		panic("extension goroutine failed")
	}()
	return []nvm.Item{{Title: "pending"}}
}
`
	case "bomb":
		return `package main

import "nvm"

func Search(q string) []nvm.Item {
	var keep [][]byte
	for {
		keep = append(keep, make([]byte, 1<<20))
	}
}
`
	}
	return TrivialGo
}
