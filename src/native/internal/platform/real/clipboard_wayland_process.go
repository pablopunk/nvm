package real

import (
	"context"
	"io"
	"os/exec"
)

// processStream is the standard output of a long-running child process. Closing it stops the process.
type processStream struct {
	io.ReadCloser
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// startProcessStream runs binary until ctx is done or the stream is closed and returns its standard output. The
// child dies with the app (see dieWithParent), so a crashed app leaves no watcher behind.
func startProcessStream(ctx context.Context, binary string, args ...string) (io.ReadCloser, error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, binary, args...)
	dieWithParent(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	return &processStream{ReadCloser: stdout, cmd: cmd, cancel: cancel}, nil
}

// Close stops the process and reaps it.
func (s *processStream) Close() error {
	s.cancel()
	_ = s.cmd.Wait()
	return nil
}
