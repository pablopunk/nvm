package fake

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// SpeechRecognizer is a platform.SpeechRecognizer that answers every request with the number of samples it heard,
// so tests can check what reached the model and when sessions open and close.
type SpeechRecognizer struct {
	mu      sync.Mutex
	opened  []string
	open    int
	samples [][]int16
	faults  faultSet
}

var _ platform.SpeechRecognizer = (*SpeechRecognizer)(nil)

// NewSpeechRecognizer returns a recognizer with no session open.
func NewSpeechRecognizer() *SpeechRecognizer {
	return &SpeechRecognizer{}
}

// FailNext makes the next call of op ("speech.open" or "speech.transcribe") return err.
func (r *SpeechRecognizer) FailNext(op string, err error) {
	r.faults.failNext(op, err)
}

// Open records dir and returns a session.
func (r *SpeechRecognizer) Open(dir string) (platform.SpeechSession, error) {
	if err := r.faults.take("speech.open"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.opened = append(r.opened, dir)
	r.open++
	return &speechSession{recognizer: r}, nil
}

// Opened lists the folders sessions were opened on, in order.
func (r *SpeechRecognizer) Opened() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.opened...)
}

// OpenSessions is how many sessions are open now.
func (r *SpeechRecognizer) OpenSessions() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.open
}

// Heard returns the PCM of every request, in order.
func (r *SpeechRecognizer) Heard() [][]int16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]int16(nil), r.samples...)
}

type speechSession struct {
	recognizer *SpeechRecognizer
	once       sync.Once
	closed     bool
}

func (s *speechSession) Transcribe(ctx context.Context, pcm []int16) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := s.recognizer.faults.take("speech.transcribe"); err != nil {
		return "", err
	}
	r := s.recognizer
	r.mu.Lock()
	defer r.mu.Unlock()
	if s.closed {
		return "", errors.New("the speech session is closed")
	}
	r.samples = append(r.samples, append([]int16(nil), pcm...))
	return fmt.Sprintf("heard %d samples", len(pcm)), nil
}

func (s *speechSession) Close() error {
	s.once.Do(func() {
		r := s.recognizer
		r.mu.Lock()
		defer r.mu.Unlock()
		s.closed = true
		r.open--
	})
	return nil
}
