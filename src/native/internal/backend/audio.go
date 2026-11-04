package backend

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Dictation limits and budget. The backend accepts at most 4 MiB of WebM per request and answers within 16 s; a
// request that runs longer is cut here so the caller shows a timeout rather than waiting on the backend's own.
const (
	maxTranscriptionBytes  = 4 << 20
	transcriptionTimeout   = 16 * time.Second
	preparationTimeout     = 16 * time.Second
	webmContentType        = "audio/webm"
	transcriptionKeyHeader = "Idempotency-Key"
)

// errMissingText marks a transcription answer without its text field.
var errMissingText = errors.New("the text field is missing")

// TranscriptionRequest is one WebM file to transcribe. OperationID keys the request's idempotency: a repeat of the
// same ID is answered from the backend's record instead of billed again.
type TranscriptionRequest struct {
	OperationID string
	Audio       []byte
}

// Transcribe sends one WebM recording and returns its text. The call is a POST and never retried by the client; a
// caller that repeats it must reuse OperationID.
func (c *Client) Transcribe(ctx context.Context, req TranscriptionRequest) (string, error) {
	if len(req.Audio) == 0 {
		return "", &Error{Kind: KindRejected, Message: "there is no recording to transcribe"}
	}
	if len(req.Audio) > maxTranscriptionBytes {
		return "", &Error{Kind: KindRejected, Message: "the recording is too large to transcribe; split it into shorter clips"}
	}
	if len(req.OperationID) == 0 || len(req.OperationID) > maxIdempotencyKeyLength {
		return "", &Error{Kind: KindRejected, Message: "transcription requests need an Idempotency-Key of at most 200 characters"}
	}
	var wire struct {
		Text *string `json:"text"`
	}
	err := c.callJSON(ctx, requestSpec{
		method:  "POST",
		path:    pathAudioTranscriptions,
		body:    req.Audio,
		header:  map[string]string{"Content-Type": webmContentType, transcriptionKeyHeader: req.OperationID},
		auth:    authRequired,
		timeout: transcriptionTimeout,
	}, &wire)
	if err != nil {
		return "", err
	}
	if wire.Text == nil {
		return "", &Error{Kind: KindBadResponse, Message: "the transcription response has no text", Cause: errMissingText}
	}
	return *wire.Text, nil
}

// PrepareTranscription asks the backend to check admission for operationID before the recording ends, so the
// transcription that follows starts sooner. The desktop app ignores its failure, and so does the caller.
func (c *Client) PrepareTranscription(ctx context.Context, operationID string) error {
	return c.operationCall(ctx, "POST", operationID, preparationTimeout)
}

// CancelTranscriptionPreparation releases the admission PrepareTranscription took for operationID.
func (c *Client) CancelTranscriptionPreparation(ctx context.Context, operationID string) error {
	return c.operationCall(ctx, "DELETE", operationID, preparationTimeout)
}

func (c *Client) operationCall(ctx context.Context, method, operationID string, timeout time.Duration) error {
	body, err := json.Marshal(struct {
		OperationID string `json:"operationId"`
	}{OperationID: operationID})
	if err != nil {
		return &Error{Kind: KindRejected, Message: "could not encode the preparation request", Cause: err}
	}
	return c.callJSON(ctx, requestSpec{
		method:  method,
		path:    pathTranscriptionPreparations,
		body:    body,
		auth:    authRequired,
		timeout: timeout,
	}, nil)
}
