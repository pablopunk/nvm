package fixtures

import (
	"encoding/json"
	"net/http"
	"strings"
)

// DecisionsPath is the decision route the AI commands call.
const DecisionsPath = "/api/v1/decisions"

// SetDecisionBody replaces the answers the decision route returns, from the next request on. The body is the JSON
// the backend sends: id, model, answers and usage. An empty body restores the default answers.
func (b *Backend) SetDecisionBody(body string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.decisionBody = body
}

// decisionReply returns the body the decision route answers with for a request body.
func (b *Backend) decisionReply(requestBody string) string {
	b.mu.Lock()
	custom := b.decisionBody
	b.mu.Unlock()
	if custom != "" {
		return custom
	}
	return answerDecisionDefaults(requestBody)
}

// answerDecisionDefaults fills in an answer for each question of a decision request by its type.
func answerDecisionDefaults(requestBody string) string {
	var request struct {
		Questions []struct {
			ID      string   `json:"id"`
			Type    string   `json:"type"`
			Options []string `json:"options"`
		} `json:"questions"`
	}
	answers := map[string]any{}
	if err := json.Unmarshal([]byte(requestBody), &request); err == nil {
		for _, question := range request.Questions {
			answers[question.ID] = defaultAnswer(question.Type, question.Options)
		}
	}
	body, _ := json.Marshal(map[string]any{
		"id": "decision-fixture", "model": "fixture-fast", "answers": answers,
		"usage": map[string]int{"inputTokens": 12, "outputTokens": 3},
	})
	return string(body)
}

// defaultAnswer is the answer the fixture gives a question of type.
func defaultAnswer(questionType string, options []string) map[string]any {
	switch questionType {
	case "predicate":
		return map[string]any{"type": "predicate", "value": true, "probability": 0.9}
	case "choice":
		first := ""
		if len(options) > 0 {
			first = options[0]
		}
		return map[string]any{"type": "choice", "value": strings.TrimSpace(first)}
	}
	return map[string]any{"type": "score", "value": 3}
}

// serveDecision answers a decision request with the configured or default body.
func (b *Backend) serveDecision(w http.ResponseWriter, body string) {
	writeBody(w, http.StatusOK, "application/json", b.decisionReply(body))
}
