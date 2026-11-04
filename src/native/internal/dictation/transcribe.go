package dictation

import (
	"context"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/dictation/webm"
)

// Segment is one WebM file of a dictation as the backend receives it. OperationID keys the request's idempotency;
// CancellationID names the whole dictation; Prepared says the backend already checked admission for OperationID.
type Segment struct {
	OperationID    string
	CancellationID string
	Prepared       bool
	File           []byte
}

// Transcriber is the backend port. Prepare warms the backend's admission before the recording ends, Transcribe sends
// one segment, and CancelPreparation releases an admission nobody will use. userCancelled marks a dictation the user
// stopped, as opposed to one that failed on its own.
type Transcriber interface {
	Prepare(ctx context.Context, operationID string) error
	Transcribe(ctx context.Context, segment Segment) (string, error)
	CancelPreparation(ctx context.Context, operationID string, userCancelled bool) error
}

// transcribeSegments sends every segment in order and joins the texts with spaces, as the desktop app does. The
// first segment is the one the preparation covers. A failed segment fails the dictation.
func transcribeSegments(ctx context.Context, transcriber Transcriber, op *operation) (string, error) {
	var texts []string
	for index, packets := range op.segments {
		file := webm.Mux(webm.Track{SampleRate: SampleRate, Channels: Channels, Lookahead: op.encoder.Lookahead()}, packets, FrameMillis)
		text, err := transcriber.Transcribe(ctx, Segment{
			OperationID:    segmentOperationID(op.id, index),
			CancellationID: op.id,
			Prepared:       index == 0 && op.preparedOK,
			File:           file,
		})
		if err != nil {
			return "", err
		}
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			texts = append(texts, trimmed)
		}
	}
	return strings.Join(texts, " "), nil
}

// segmentOperationID keeps the first segment's ID as the operation's, so the backend sees the key the preparation
// used, and gives later segments a suffix.
func segmentOperationID(operationID string, index int) string {
	if index == 0 {
		return operationID
	}
	return fmt.Sprintf("%s-%d", operationID, index)
}
