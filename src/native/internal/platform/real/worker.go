package real

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// defaultMaxWorkerLineBytes applies when a spec leaves MaxLineBytes at zero.
const defaultMaxWorkerLineBytes = 1 << 20

// NewWorker returns the production helper runner.
func NewWorker() platform.Worker {
	return worker{}
}

type worker struct{}

// Start launches the helper with pipes on its standard streams. The helper is not tied to any context:
// its owner kills it.
func (worker) Start(spec platform.WorkerSpec) (platform.WorkerProcess, error) {
	cmd := exec.Command(spec.Binary, spec.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &tailBuffer{limit: platform.MaxWorkerStderrBytes}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	maxLine := spec.MaxLineBytes
	if maxLine <= 0 {
		maxLine = defaultMaxWorkerLineBytes
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, min(64<<10, maxLine)), maxLine)
	return &workerProcess{cmd: cmd, stdin: stdin, lines: scanner, stderr: stderr}, nil
}

type workerProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  *bufio.Scanner
	stderr *tailBuffer
	reap   sync.Once
	code   int
	waitE  error
}

func (p *workerProcess) PID() int {
	return p.cmd.Process.Pid
}

func (p *workerProcess) WriteLine(line []byte) error {
	framed := append(bytes.Clone(line), '\n')
	_, err := p.stdin.Write(framed)
	return err
}

// ReadLine returns a copy of the next line, because the scanner reuses its buffer.
func (p *workerProcess) ReadLine() ([]byte, error) {
	if !p.lines.Scan() {
		if err := p.lines.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	return bytes.Clone(p.lines.Bytes()), nil
}

// Kill ends the process. A process that already exited is not an error.
func (p *workerProcess) Kill() error {
	err := p.cmd.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

// Wait reaps the process once. It must not run while ReadLine is still reading, as exec documents.
func (p *workerProcess) Wait() (int, error) {
	p.reap.Do(func() {
		p.waitE = p.cmd.Wait()
		p.code = p.cmd.ProcessState.ExitCode()
	})
	return p.code, p.waitE
}

func (p *workerProcess) StderrTail() string {
	return p.stderr.String()
}

// tailBuffer keeps the last limit bytes written to it. It is written by the exec copier goroutine and read
// by the owner, so it locks.
type tailBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if extra := len(b.data) - b.limit; extra > 0 {
		b.data = b.data[extra:]
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}
