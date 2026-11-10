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

// Recorder keeps a dictation's audio until its transcript is in, so a dictation that failed can be transcribed again
// from the history. Save reports a failure the dictation survives without; Remove drops the audio once the text is
// confirmed; Load returns the saved segments of one dictation.
type Recorder interface {
	Save(id string, segments [][]byte) error
	Remove(id string) error
	Load(id string) ([][]byte, error)
}

// SavedError is a failed transcription whose audio was kept, so the user can transcribe it again from the history.
type SavedError struct {
	Err error
}

func (e *SavedError) Error() string { return e.Err.Error() }

// Unwrap returns the transcription's own error.
func (e *SavedError) Unwrap() error { return e.Err }

// segmentFiles muxes every captured segment of op into a WebM file, in order.
func segmentFiles(op *operation) [][]byte {
	track := webm.Track{SampleRate: SampleRate, Channels: Channels, Lookahead: op.encoder.Lookahead()}
	files := make([][]byte, 0, len(op.segments))
	for _, packets := range op.segments {
		files = append(files, webm.Mux(track, packets, FrameMillis))
	}
	return files
}

// transcribeFiles sends every segment file in order and joins the texts with spaces, as the desktop app does. The
// first segment is the one a preparation covers when prepared is set. A failed segment fails the dictation.
func transcribeFiles(ctx context.Context, transcriber Transcriber, operationID string, prepared bool, files [][]byte) (string, error) {
	var texts []string
	for index, file := range files {
		text, err := transcriber.Transcribe(ctx, Segment{
			OperationID:    segmentOperationID(operationID, index),
			CancellationID: operationID,
			Prepared:       index == 0 && prepared,
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
