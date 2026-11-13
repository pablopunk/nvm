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
	operationIDHeader      = "X-Nevermind-Dictation-Operation-ID"
	preparationHeader      = "X-Nevermind-Dictation-Preparation"
	cancellationReasonHdr  = "X-Nevermind-Dictation-Cancellation-Reason"
	userCancelReason       = "user_cancel"
)

// errMissingText marks a transcription answer without its text field.
var errMissingText = errors.New("the text field is missing")

// TranscriptionRequest is one WebM file to transcribe. OperationID keys the request's idempotency: a repeat of the
// same ID is answered from the backend's record instead of billed again. CancellationID names the whole dictation
// the file belongs to, so the backend can tie a segment to its preparation and cancellation. Prepared says the
// backend already checked admission for OperationID through PrepareTranscription.
type TranscriptionRequest struct {
	OperationID    string
	CancellationID string
	Prepared       bool
	Audio          []byte
}

// Transcribe sends one WebM recording and returns its text. The call is a POST and never retried by the client; a
// caller that repeats it must reuse OperationID.
func (c *Client) Transcribe(ctx context.Context, req TranscriptionRequest) (string, error) {
	if err := validateTranscription(req); err != nil {
		return "", err
	}
	var wire struct {
		Text *string `json:"text"`
	}
	err := c.callJSON(ctx, requestSpec{
		method:  "POST",
		path:    pathAudioTranscriptions,
		body:    req.Audio,
		header:  transcriptionHeaders(req),
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

// validateTranscription refuses an unusable recording before any request is made.
func validateTranscription(req TranscriptionRequest) error {
	switch {
	case len(req.Audio) == 0:
		return &Error{Kind: KindRejected, Message: "there is no recording to transcribe"}
	case len(req.Audio) > maxTranscriptionBytes:
		return &Error{Kind: KindRejected, Message: "the recording is too large to transcribe; split it into shorter clips"}
	case len(req.OperationID) == 0 || len(req.OperationID) > maxIdempotencyKeyLength:
		return &Error{Kind: KindRejected, Message: "transcription requests need an Idempotency-Key of at most 200 characters"}
	}
	return nil
}

// transcriptionHeaders are the request headers the desktop app sends: the content type, the idempotency key, the
// dictation the file belongs to, and the preparation the backend should reuse when there is one.
func transcriptionHeaders(req TranscriptionRequest) map[string]string {
	headers := map[string]string{"Content-Type": webmContentType, transcriptionKeyHeader: req.OperationID}
	if req.CancellationID != "" {
		headers[operationIDHeader] = req.CancellationID
	}
	if req.Prepared {
		headers[preparationHeader] = req.OperationID
	}
	return headers
}

// PrepareTranscription asks the backend to check admission for operationID before the recording ends, so the
// transcription that follows starts sooner. The caller ignores its failure, as the desktop app does.
func (c *Client) PrepareTranscription(ctx context.Context, operationID string) error {
	return c.preparationCall(ctx, "POST", operationID, nil)
}

// CancelTranscriptionPreparation releases the admission PrepareTranscription took for operationID. userCancelled
// marks a dictation the user stopped, as opposed to one that failed on its own.
func (c *Client) CancelTranscriptionPreparation(ctx context.Context, operationID string, userCancelled bool) error {
	var header map[string]string
	if userCancelled {
		header = map[string]string{cancellationReasonHdr: userCancelReason}
	}
	return c.preparationCall(ctx, "DELETE", operationID, header)
}

func (c *Client) preparationCall(ctx context.Context, method, operationID string, header map[string]string) error {
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
		header:  header,
		auth:    authRequired,
		timeout: preparationTimeout,
	}, nil)
}
