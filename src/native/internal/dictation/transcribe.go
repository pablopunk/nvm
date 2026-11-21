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

// Transcriber turns segments into text: the backend, or the model on this device. Prepare readies it when the
// microphone opens (the backend's admission, the device's model load), Transcribe sends one segment, CancelPreparation
// releases a preparation nobody will use, and Finished says the dictation is over, whatever its outcome, so held
// resources can go. userCancelled marks a dictation the user stopped, as opposed to one that failed on its own.
type Transcriber interface {
	Prepare(ctx context.Context, operationID string) error
	Transcribe(ctx context.Context, segment Segment) (string, error)
	CancelPreparation(ctx context.Context, operationID string, userCancelled bool) error
	Finished(operationID string)
}

// Recorder keeps a dictation's audio on disk from its first captured frame until its transcript is in, so a dictation
// that failed, was cancelled or was cut off by a crash can be transcribed again from the history. SaveSegment writes
// one WebM segment, replacing an earlier copy of it, and lists the dictation from its first call; Remove drops the audio
// once the text is confirmed; Load returns the saved segments of one dictation.
type Recorder interface {
	SaveSegment(id string, index int, audio []byte) error
	// MarkFailed records why the transcription failed, so the history can say so after a restart.
	MarkFailed(id, failure string) error
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

// wholeRecordingReader is a transcriber that reads a dictation in one pass at Stop instead of piece by piece while the
// user speaks, as the on-device model does: it is fast enough that the wait stays short, and seeing the whole
// recording keeps it from ending a sentence at every pause.
type wholeRecordingReader interface {
	ReadsWholeRecording() bool
}

func readsWholeRecording(t Transcriber) bool {
	reader, ok := t.(wholeRecordingReader)
	return ok && reader.ReadsWholeRecording()
}

// segmentFiles muxes every captured segment of op into a WebM file, in order.
func segmentFiles(op *operation) [][]byte {
	track := op.track()
	files := make([][]byte, 0, len(op.segments))
	for _, packets := range op.segments {
		files = append(files, webm.Mux(track, packets, FrameMillis))
	}
	return files
}

// transcribeFiles sends every segment file in order and joins the texts with spaces, as the desktop app does. The
// first segment is the one a preparation covers when prepared is set. A segment that fails for a transient reason is retried
// under policy; any other failure fails the dictation.
func transcribeFiles(ctx context.Context, transcriber Transcriber, policy RetryPolicy, operationID string, prepared bool, files [][]byte, onRetry func(int, error)) (string, error) {
	var texts []string
	for index, file := range files {
		text, err := transcribeSegment(ctx, transcriber, policy, Segment{
			OperationID:    segmentOperationID(operationID, index),
			CancellationID: operationID,
			Prepared:       index == 0 && prepared,
			File:           file,
		}, onRetry)
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
