package real

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/parakeet"
)

// errSpeechSessionClosed is a request to a session that was closed or whose child ended.
var errSpeechSessionClosed = errors.New("the on-device speech model stopped")

// speechRecognizer runs the on-device model in a child process of the app, on the transcribe.cpp library the app
// ships (Hex's engine), or on library when it is set.
type speechRecognizer struct {
	library string
}

// NewSpeechRecognizer runs the on-device model on the transcribe.cpp library at library, for tools that run outside the
// packaged app. The binary that calls it must serve parakeet.Main, since the child is the same binary.
func NewSpeechRecognizer(library string) platform.SpeechRecognizer {
	return speechRecognizer{library: library}
}

// Open starts the child that loads the model in dir.
func (r speechRecognizer) Open(dir string) (platform.SpeechSession, error) {
	library := r.library
	if library == "" {
		library = existingBundledResource("lib", transcribeLibraryName(runtime.GOOS))
	}
	if library == "" {
		return nil, fmt.Errorf("%w: this build does not ship the speech runtime", platform.ErrUnsupported)
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return startSpeechChild(exec.Command(executable, parakeet.ChildFlag, library, dir))
}

// speechChild is one running child. Requests go out one at a time; the replies come back in order.
type speechChild struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	replies *bufio.Reader
	stderr  *boundedBuffer
	ready   chan struct{}
	// loadErr is set before ready closes when the model could not be loaded.
	loadErr error

	mu        sync.Mutex
	closeOnce sync.Once
}

func startSpeechChild(cmd *exec.Cmd) (*speechChild, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	child := &speechChild{cmd: cmd, stdin: stdin, replies: bufio.NewReader(stdout), stderr: &boundedBuffer{limit: 4096}, ready: make(chan struct{})}
	cmd.Stderr = child.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start the speech model: %w", err)
	}
	go child.awaitReady()
	return child, nil
}

// awaitReady reads the child's first reply, which says whether the model loaded.
func (c *speechChild) awaitReady() {
	defer close(c.ready)
	reply, err := c.readReply()
	switch {
	case err != nil:
		c.loadErr = err
	case reply.Error != "":
		c.loadErr = fmt.Errorf("load the speech model: %s", reply.Error)
	case !reply.Ready:
		c.loadErr = errors.New("the speech model answered before it was ready")
	}
	if c.loadErr != nil {
		_ = c.Close()
	}
}

// Transcribe sends pcm and waits for its text. A cancelled request ends the child, since a request cannot be taken
// back; the next session starts a new one.
func (c *speechChild) Transcribe(ctx context.Context, pcm []int16) (string, error) {
	select {
	case <-c.ready:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if c.loadErr != nil {
		return "", c.loadErr
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	type answer struct {
		reply parakeet.Reply
		err   error
	}
	done := make(chan answer, 1)
	go func() {
		if err := parakeet.WriteRequest(c.stdin, pcm); err != nil {
			done <- answer{err: c.failure(err)}
			return
		}
		reply, err := c.readReply()
		done <- answer{reply, err}
	}()
	select {
	case got := <-done:
		if got.err != nil {
			return "", got.err
		}
		if got.reply.Error != "" {
			return "", fmt.Errorf("transcribe on the device: %s", got.reply.Error)
		}
		return got.reply.Text, nil
	case <-ctx.Done():
		_ = c.Close()
		<-done
		return "", ctx.Err()
	}
}

func (c *speechChild) readReply() (parakeet.Reply, error) {
	line, err := c.replies.ReadBytes('\n')
	if err != nil {
		return parakeet.Reply{}, c.failure(err)
	}
	var reply parakeet.Reply
	if err := json.Unmarshal(line, &reply); err != nil {
		return parakeet.Reply{}, fmt.Errorf("the speech model answered %q: %w", strings.TrimSpace(string(line)), err)
	}
	return reply, nil
}

// failure explains a broken pipe with what the child printed before it ended.
func (c *speechChild) failure(err error) error {
	if detail := strings.TrimSpace(c.stderr.String()); detail != "" {
		return fmt.Errorf("%w: %s", errSpeechSessionClosed, detail)
	}
	return fmt.Errorf("%w: %v", errSpeechSessionClosed, err)
}

// Close ends the child, which gives the model's memory back.
func (c *speechChild) Close() error {
	c.closeOnce.Do(func() {
		_ = c.stdin.Close()
		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		go func() { _ = c.cmd.Wait() }()
	})
	return nil
}

// boundedBuffer keeps the first limit bytes written to it, for an error message.
type boundedBuffer struct {
	mu    sync.Mutex
	limit int
	buf   bytes.Buffer
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if room := b.limit - b.buf.Len(); room > 0 {
		b.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// transcribeLibraryName is the file name of the transcribe.cpp library scripts/natives/build-transcribe.sh ships.
func transcribeLibraryName(goos string) string {
	switch goos {
	case "darwin":
		return "libtranscribe.dylib"
	case "windows":
		return "transcribe.dll"
	}
	return "libtranscribe.so"
}
