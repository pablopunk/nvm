package extai

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Decide asks the decision model the questions of request about its input and attachments. The attachments are read
// strictly: one the decision cannot read fails the call.
func (s *Service) Decide(ctx context.Context, request api.DecisionRequest) (api.DecisionResult, error) {
	if !s.Available() {
		return api.DecisionResult{}, chat.ErrNotSignedIn
	}
	client := s.client()
	if client == nil {
		return api.DecisionResult{}, chat.ErrNotSignedIn
	}
	ctx, cancel := withDeadline(ctx, s.deps.Clock, CallTimeout)
	defer cancel()
	content, err := s.readAttachments(ctx, request.Attachments, true)
	if err != nil {
		return api.DecisionResult{}, err
	}
	result, err := client.Decisions(ctx, backend.DecisionRequest{
		Input:     joinNonEmpty("\n\n", inputText(request.Input), content.text()),
		Images:    content.images,
		Questions: questionsOf(request.Questions),
	})
	if err != nil {
		return api.DecisionResult{}, callError(ctx, err)
	}
	return decisionResultOf(result), nil
}

// inputText is the decision input as the text the model reads: a JSON string is its text, any other JSON is sent as
// written.
func inputText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return string(raw)
}

// joinNonEmpty joins the parts that are not empty.
func joinNonEmpty(separator string, parts ...string) string {
	var out string
	for _, part := range parts {
		if part == "" {
			continue
		}
		if out != "" {
			out += separator
		}
		out += part
	}
	return out
}

func questionsOf(questions []api.DecisionQuestion) []backend.DecisionQuestion {
	out := make([]backend.DecisionQuestion, 0, len(questions))
	for _, question := range questions {
		out = append(out, backend.DecisionQuestion{
			ID:       question.ID,
			Type:     string(question.Type),
			Question: question.Question,
			Options:  question.Options,
			Min:      question.Min,
			Max:      question.Max,
			Labels:   question.Labels,
		})
	}
	return out
}

func decisionResultOf(result backend.DecisionResult) api.DecisionResult {
	out := api.DecisionResult{ID: result.ID, Model: result.Model, Answers: map[string]api.DecisionAnswer{}}
	out.Usage.InputTokens = result.InputTokens
	out.Usage.OutputTokens = result.OutputTokens
	for id, answer := range result.Answers {
		out.Answers[id] = api.DecisionAnswer{
			Type:        api.DecisionQuestionType(answer.Type),
			Flag:        answer.Predicate,
			Probability: answer.Probability,
			Choice:      answer.Choice,
			Score:       answer.Score,
		}
	}
	return out
}
