package dictation

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/pablopunk/nvm/native/internal/dictation/webm"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// liveTranscription sends each finished segment of a recording to the backend while the user is still speaking, so
// Stop only waits for the words after the last pause. Segments are sent in parallel and joined in order. The first
// waits for the backend preparation, which covers it alone, and every segment waits until its audio is on disk.
type liveTranscription struct {
	ctx         context.Context
	transcriber Transcriber
	policy      RetryPolicy
	onRetry     func(int, error)
	operationID string
	prepared    <-chan struct{}
	preparedOK  *bool
	journal     *journal

	wg    sync.WaitGroup
	mu    sync.Mutex
	texts map[int]string
	err   error
	errAt int
}

func newLiveTranscription(s *Service, op *operation) *liveTranscription {
	return &liveTranscription{
		ctx:         op.ctx,
		transcriber: op.transcriber,
		policy:      s.cfg.Retry,
		onRetry:     s.retryWarning("dictation.transcribe-retry"),
		operationID: op.id,
		prepared:    op.prepared,
		preparedOK:  &op.preparedOK,
		journal:     op.journal,
		texts:       map[int]string{},
	}
}

// send starts transcribing segment index. packets must not change afterwards.
func (l *liveTranscription) send(index int, track webm.Track, packets [][]byte) {
	if l == nil {
		return
	}
	l.wg.Add(1)
	safego.Go("dictation.transcribe-segment", func(context.Context) {
		defer l.wg.Done()
		l.journal.awaitSaved(l.ctx, index, len(packets))
		text, err := l.transcribe(index, webm.Mux(track, packets, FrameMillis))
		l.record(index, text, err)
	})
}

func (l *liveTranscription) transcribe(index int, file []byte) (string, error) {
	select {
	case <-l.prepared:
	case <-l.ctx.Done():
	}
	if err := l.ctx.Err(); err != nil {
		return "", err
	}
	return transcribeSegment(l.ctx, l.transcriber, l.policy, Segment{
		OperationID:    segmentOperationID(l.operationID, index),
		CancellationID: l.operationID,
		Prepared:       index == 0 && *l.preparedOK,
		File:           file,
	}, l.onRetry)
}

// record keeps a segment's text, or the failure of the earliest segment that failed.
func (l *liveTranscription) record(index int, text string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err != nil {
		if l.err == nil || index < l.errAt {
			l.err, l.errAt = err, index
		}
		return
	}
	l.texts[index] = strings.TrimSpace(text)
}

// wait returns the texts of segments 0..count-1 joined with spaces, once every segment sent is answered.
func (l *liveTranscription) wait(count int) (string, error) {
	l.wg.Wait()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return "", l.err
	}
	texts := make([]string, 0, count)
	for index := range count {
		text, ok := l.texts[index]
		if !ok {
			return "", errMissingSegment
		}
		if text != "" {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, " "), nil
}

var errMissingSegment = errors.New("a segment of the dictation was never transcribed")
