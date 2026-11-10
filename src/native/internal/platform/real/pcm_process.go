package real

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// pcmProcessStream reads the raw PCM a child process writes to stdout. An odd byte left over from one read waits for
// the next. classify turns the child's stderr into the error a Read returns once the child has ended without samples.
type pcmProcessStream struct {
	cmd      *exec.Cmd
	out      *bufio.Reader
	stderr   *bytes.Buffer
	carry    []byte
	reaped   sync.Once
	classify func(message string) error
}

// newPCMProcessStream wraps a started child whose stdout carries signed 16-bit little-endian samples.
func newPCMProcessStream(cmd *exec.Cmd, stdout io.Reader, stderr *bytes.Buffer, classify func(message string) error) *pcmProcessStream {
	return &pcmProcessStream{cmd: cmd, out: bufio.NewReaderSize(stdout, 1<<16), stderr: stderr, classify: classify}
}

// reap waits for the child once, however many paths ask. Close must not wait on it: a child that is killed while a Read
// is pending has been seen not to return from Wait, and Stop would then hang behind the capture.
func (s *pcmProcessStream) reap() {
	s.reaped.Do(func() { _ = s.cmd.Wait() })
}

// Read fills pcm with samples, blocking until the child writes some. When the child exits, the stream ends with
// io.EOF, or with the error its stderr classifies to when it failed.
func (s *pcmProcessStream) Read(pcm []int16) (int, error) {
	want := 2 * len(pcm)
	buf := make([]byte, want)
	copy(buf, s.carry)
	n, err := io.ReadAtLeast(s.out, buf[len(s.carry):], max(0, want-len(s.carry)))
	total := len(s.carry) + n
	s.carry = nil
	if err != nil && total == 0 {
		return 0, s.endError(err)
	}
	samples := total / 2
	for i := range samples {
		pcm[i] = int16(binary.LittleEndian.Uint16(buf[2*i:]))
	}
	s.carry = append([]byte(nil), buf[2*samples:total]...)
	return samples, nil
}

func (s *pcmProcessStream) endError(readErr error) error {
	if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
		s.reap()
		if message := strings.TrimSpace(s.stderr.String()); message != "" {
			return s.classify(message)
		}
		return io.EOF
	}
	return readErr
}

// Close kills the child, which ends a pending Read with the stream's end. It returns without waiting for the process.
func (s *pcmProcessStream) Close() error {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	go s.reap()
	return nil
}
