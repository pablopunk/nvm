package emoji

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ai/charsearch"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/emoji"
)

// meaning answers the typed text with the characters the model finds by meaning, after the catalog's own matches, which
// it does not repeat. Without an account the batch is empty, as the 1.x picker has no batch while signed out.
func (e *Extension) meaning(ctx api.Context, in api.Invocation) (api.Result, error) {
	text := strings.TrimSpace(in.Value)
	category := categoryOf(in.Ref.Args)
	idx := e.catalog(ctx)
	empty := api.Result{Patch: &api.ViewPatch{}}
	if text == "" || idx.matcher == nil {
		return empty, nil
	}
	decider, ok := e.modelFor(ctx)
	if !ok {
		return empty, nil
	}
	matches, err := idx.matcher.Matches(ctx.Std(), decider, category, text)
	if err != nil {
		return empty, nil
	}
	return api.Result{Patch: &api.ViewPatch{SearchItems: meaningItems(idx, matches, e.tone(ctx), canPaste(ctx), text, catalogIDs(category, text))}}, nil
}

// modelFor is the decision client of the meaning batch: the injected one, or the account's model, which is absent while
// the account is signed out.
func (e *Extension) modelFor(ctx api.Context) (charsearch.Decider, bool) {
	if e.decider != nil {
		return e.decider, true
	}
	available, err := ctx.AI().IsAvailable()
	if err != nil || !available {
		return nil, false
	}
	return decisionClient{ai: ctx.AI()}, true
}

// meaningItems are the rows of the model's matches, best first, with the skin tone the picker shows, skipping the
// characters the catalog already matched.
func meaningItems(idx *index, matches []charsearch.Record, tone emoji.SkinTone, paste bool, query string, shown map[string]bool) []api.Item {
	items := make([]api.Item, 0, len(matches))
	for _, match := range matches {
		record, ok := idx.recordOf(match.Key)
		if !ok || shown[match.Key] {
			continue
		}
		items = append(items, characterItem(record, tone, paste, query))
	}
	return items
}

// catalogIDs is the set of characters the catalog matches for text in category, which the meaning batch leaves out.
func catalogIDs(category, text string) map[string]bool {
	entries := characterLibrary.Search(text, emoji.Options{Category: category, Limit: RowLimit})
	ids := make(map[string]bool, len(entries))
	for _, entry := range entries {
		ids[entry.ID()] = true
	}
	return ids
}

// decisionClient runs the matcher's decisions through the host's model calls, which count against the picker's quota.
type decisionClient struct {
	ai api.AI
}

// Decisions sends one decision request and converts the answer to the backend's shape, which the matcher reads.
func (c decisionClient) Decisions(_ context.Context, req backend.DecisionRequest) (backend.DecisionResult, error) {
	input, err := json.Marshal(req.Input)
	if err != nil {
		return backend.DecisionResult{}, err
	}
	questions := make([]api.DecisionQuestion, 0, len(req.Questions))
	for _, question := range req.Questions {
		questions = append(questions, api.DecisionQuestion{
			ID: question.ID, Type: api.DecisionQuestionType(question.Type), Question: question.Question,
			Options: question.Options, Min: question.Min, Max: question.Max, Labels: question.Labels,
		})
	}
	result, err := c.ai.Decide(api.DecisionRequest{Input: input, Questions: questions})
	if err != nil {
		return backend.DecisionResult{}, err
	}
	return decisionResult(result), nil
}

// decisionResult converts an answer from the host's decision shape to the backend's.
func decisionResult(result api.DecisionResult) backend.DecisionResult {
	answers := make(map[string]backend.DecisionAnswer, len(result.Answers))
	for id, answer := range result.Answers {
		answers[id] = backend.DecisionAnswer{
			Type: string(answer.Type), Predicate: answer.Flag, Probability: answer.Probability, Choice: answer.Choice,
			Score: answer.Score, Probabilities: answer.Probabilities, Confidence: answer.Confidence,
		}
	}
	return backend.DecisionResult{
		ID: result.ID, Model: result.Model, Answers: answers,
		InputTokens: result.Usage.InputTokens, OutputTokens: result.Usage.OutputTokens,
	}
}
