package backend

import (
	"context"
	"encoding/json"
	"time"
)

// decisionsTimeout matches the backend's decision route budget.
const decisionsTimeout = 120 * time.Second

// maxIdempotencyKeyLength is the longest Idempotency-Key the backend accepts.
const maxIdempotencyKeyLength = 200

// DecisionImage is one image attached to a decision, base64 encoded.
type DecisionImage struct {
	// MediaType is image/png, image/jpeg or image/webp.
	MediaType string `json:"mediaType"`
	// Data is the base64 image bytes, at most 2 MiB decoded.
	Data string `json:"data"`
}

// DecisionQuestion is one question the decision model answers. Only the fields for its Type are sent.
type DecisionQuestion struct {
	// ID is the question key in the answers map: letters, digits, underscore or hyphen, up to 64 characters.
	ID string `json:"id"`
	// Type is "predicate" (yes or no with a probability), "choice" (one of Options) or "score" (an integer
	// between Min and Max).
	Type string `json:"type"`
	// Question is the text asked of the model.
	Question string `json:"question"`
	// Options lists the choices for a choice question.
	Options []string `json:"options,omitempty"`
	// Min is the lowest score for a score question.
	Min *int `json:"min,omitempty"`
	// Max is the highest score for a score question.
	Max *int `json:"max,omitempty"`
	// Labels optionally names score levels.
	Labels map[string]string `json:"labels,omitempty"`
}

// DecisionRequest asks the decision model to answer questions about an input.
type DecisionRequest struct {
	// Input is the text the questions are about.
	Input string
	// Images are optional attachments, at most four.
	Images []DecisionImage
	// Questions are the questions to answer, at least one.
	Questions []DecisionQuestion
	// IdempotencyKey lets a caller repeat the same decision without being billed twice. Empty generates a new key.
	IdempotencyKey string
}

// decisionRequestWire is the body the route receives. The key travels in a header, not in the body.
type decisionRequestWire struct {
	Input     string             `json:"input"`
	Images    []DecisionImage    `json:"images"`
	Questions []DecisionQuestion `json:"questions"`
}

// DecisionResult is the decision model's answers.
type DecisionResult struct {
	// ID identifies the decision on the backend.
	ID string
	// Model is the model that answered.
	Model string
	// Answers maps each question ID to its typed answer.
	Answers map[string]DecisionAnswer
	// InputTokens and OutputTokens count the tokens billed for the decision.
	InputTokens  int
	OutputTokens int
}

// Decisions asks the decision model the questions in req. The call is a POST, so it is never retried by the
// client; a caller that retries should reuse IdempotencyKey, which the backend dedups.
func (c *Client) Decisions(ctx context.Context, req DecisionRequest) (DecisionResult, error) {
	key := req.IdempotencyKey
	if key == "" {
		key = newRequestID()
	}
	if key == "" || len(key) > maxIdempotencyKeyLength {
		return DecisionResult{}, &Error{Kind: KindRejected, Message: "decision requests need an Idempotency-Key of at most 200 characters"}
	}
	body, err := json.Marshal(decisionRequestWire{
		Input:     req.Input,
		Images:    nonNilSlice(req.Images),
		Questions: nonNilSlice(req.Questions),
	})
	if err != nil {
		return DecisionResult{}, &Error{Kind: KindRejected, Message: "could not encode the decision request", Cause: err}
	}
	var wire decisionWire
	err = c.callJSON(ctx, requestSpec{
		method:  "POST",
		path:    pathDecisions,
		body:    body,
		header:  map[string]string{"Idempotency-Key": key},
		auth:    authRequired,
		timeout: decisionsTimeout,
	}, &wire)
	if err != nil {
		return DecisionResult{}, err
	}
	return wire.result()
}

// nonNilSlice returns s, or an empty slice when s is nil, so the JSON carries [] rather than null.
func nonNilSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
