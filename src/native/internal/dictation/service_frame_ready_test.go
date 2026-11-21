package dictation

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

type gatedFrameMic struct {
	*fake.Microphone
	stream *gatedFrameStream
}

func (m gatedFrameMic) Open(context.Context, string, platform.AudioFormat) (platform.MicrophoneStream, error) {
	return m.stream, nil
}

type gatedFrameStream struct {
	reading   chan struct{}
	release   chan struct{}
	closed    chan struct{}
	once      sync.Once
	closeOnce sync.Once
	delivered bool
}

func (s *gatedFrameStream) Read(p []int16) (int, error) {
	s.once.Do(func() { close(s.reading) })
	select {
	case <-s.release:
		if s.delivered {
			<-s.closed
			return 0, io.EOF
		}
		s.delivered = true
		select {
		case <-s.closed:
			return 0, io.EOF
		default:
		}
		clear(p)
		return len(p), nil
	case <-s.closed:
		return 0, io.EOF
	}
}
func (s *gatedFrameStream) Close() error { s.closeOnce.Do(func() { close(s.closed) }); return nil }
func newGatedFrameStream() *gatedFrameStream {
	return &gatedFrameStream{reading: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{})}
}
func TestOrdinaryMicrophoneStartWaitsForCapturedSilentFrame(t *testing.T) {
	h := newHarness(t)
	stream := newGatedFrameStream()
	h.service.cfg.Microphone = gatedFrameMic{h.mic, stream}
	result := make(chan error, 1)
	go func() { result <- h.service.Start("") }()
	<-stream.reading
	select {
	case err := <-result:
		t.Fatalf("start returned before a frame: %v", err)
	default:
	}
	close(stream.release)
	if err := <-result; err != nil {
		t.Fatalf("start after frame: %v", err)
	}
	h.service.Cancel()
}
func TestCancelBeforeFirstFrameNeverStarts(t *testing.T) {
	h := newHarness(t)
	stream := newGatedFrameStream()
	h.service.cfg.Microphone = gatedFrameMic{h.mic, stream}
	result := make(chan error, 1)
	go func() { result <- h.service.Start("") }()
	<-stream.reading
	h.service.Cancel()
	close(stream.release)
	if err := <-result; !errors.Is(err, ErrCancelled) {
		t.Fatalf("start after cancel: %v", err)
	}
}
func TestOrdinaryMicrophoneEOFBeforeFrameFails(t *testing.T) {
	h := newHarness(t)
	h.service.cfg.Retry.Attempts = 1
	if err := h.service.Start(""); !errors.Is(err, ErrNotReady) {
		t.Fatalf("start after EOF: %v", err)
	}
}
func TestOrdinaryMicrophoneWithoutFramesTimesOut(t *testing.T) {
	h := newHarness(t)
	h.service.cfg.Microphone = hangingMicrophone{Microphone: h.mic}
	h.service.cfg.Retry.Attempts = 1
	h.service.cfg.ReadyTimeout = time.Millisecond
	if err := h.service.Start(""); !errors.Is(err, ErrNotReady) {
		t.Fatalf("start after timeout: %v", err)
	}
}
