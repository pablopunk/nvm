package supervisor

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// wireTiming observes only byte counts and Linux process CPU ticks, never protocol contents.
type wireTiming struct {
	mu                                   sync.Mutex
	reads, writes, readBytes, writeBytes int
	cpuTicks                             uint64
}

type timingWorker struct {
	platform.Worker
	timing *wireTiming
}
type timingProcess struct {
	platform.WorkerProcess
	timing *wireTiming
}

func newTimingWorker() (*timingWorker, *wireTiming) {
	t := &wireTiming{}
	return &timingWorker{Worker: real.NewWorker(), timing: t}, t
}

func (w *timingWorker) Start(spec platform.WorkerSpec) (platform.WorkerProcess, error) {
	p, err := w.Worker.Start(spec)
	if err != nil {
		return nil, err
	}
	return &timingProcess{WorkerProcess: p, timing: w.timing}, nil
}

func (p *timingProcess) sample() {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", p.PID()))
	if err != nil {
		return
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) <= 12 {
		return
	}
	u, e1 := strconv.ParseUint(fields[11], 10, 64)
	s, e2 := strconv.ParseUint(fields[12], 10, 64)
	if e1 == nil && e2 == nil {
		p.timing.mu.Lock()
		p.timing.cpuTicks = u + s
		p.timing.mu.Unlock()
	}
}

func (p *timingProcess) ReadLine() ([]byte, error) {
	line, err := p.WorkerProcess.ReadLine()
	p.sample()
	p.timing.mu.Lock()
	p.timing.reads++
	p.timing.readBytes += len(line)
	p.timing.mu.Unlock()
	return line, err
}
func (p *timingProcess) WriteLine(line []byte) error {
	err := p.WorkerProcess.WriteLine(line)
	p.sample()
	p.timing.mu.Lock()
	p.timing.writes++
	p.timing.writeBytes += len(line)
	p.timing.mu.Unlock()
	return err
}
func (p *timingProcess) Kill() error { p.sample(); return p.WorkerProcess.Kill() }

func (w *wireTiming) report(t *testing.T) {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	t.Logf("screenshots wire read_lines=%d read_bytes=%d write_lines=%d write_bytes=%d child_cpu_ticks=%d (Linux USER_HZ)", w.reads, w.readBytes, w.writes, w.writeBytes, w.cpuTicks)
}

// screenshotTiming records aggregate host time without retaining service arguments or replies.
type screenshotTiming struct {
	ServiceHandler
	mu      sync.Mutex
	calls   int
	elapsed time.Duration
}

func (s *screenshotTiming) Serve(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply {
	start := time.Now()
	reply := s.ServiceHandler.Serve(ctx, call)
	s.mu.Lock()
	s.calls++
	s.elapsed += time.Since(start)
	s.mu.Unlock()
	return reply
}

func (s *screenshotTiming) report(t *testing.T, r *Runtime, start time.Time, phase string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	t.Logf("screenshots timing phase=%s wall=%s completed_host_calls=%d host_wall=%s slot_occupied=%t state=%s", phase, time.Since(start), s.calls, s.elapsed, len(r.slots) != 0, r.Status().State)
}
