package dictation

import (
	"context"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/dictation/webm"
)

// Transcriber sends one WebM file to the backend and returns its text. operationID keys the request's idempotency,
// so a retry of the same file is not billed twice; it must be unique per file.
type Transcriber interface {
	Transcribe(ctx context.Context, operationID string, file []byte) (string, error)
	// Prepare warms the backend's admission for operationID before the recording ends. Its failure does not stop
	// the dictation: the transcription request still goes through admission itself.
	Prepare(ctx context.Context, operationID string) error
}

// transcribeSegments sends every segment in order and joins the texts with spaces, as the desktop app does. A
// failed segment fails the dictation; segments before it are dropped, since the user cannot paste half a sentence
// without knowing where it stops.
func transcribeSegments(ctx context.Context, transcriber Transcriber, op *operation) (string, error) {
	var texts []string
	for index, packets := range op.segments {
		file := webm.Mux(webm.Track{SampleRate: SampleRate, Channels: Channels, Lookahead: op.encoder.Lookahead()}, packets, FrameMillis)
		text, err := transcriber.Transcribe(ctx, segmentOperationID(op.id, index), file)
		if err != nil {
			return "", err
		}
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			texts = append(texts, trimmed)
		}
	}
	return strings.Join(texts, " "), nil
}

// segmentOperationID keeps the first segment's ID as the operation's, so the backend sees the same key the start
// request prepared, and gives later segments a suffix.
func segmentOperationID(operationID string, index int) string {
	if index == 0 {
		return operationID
	}
	return fmt.Sprintf("%s-%d", operationID, index)
}
