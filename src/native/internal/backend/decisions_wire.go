package backend

import (
	"encoding/json"
	"fmt"
)

// DecisionAnswer is one typed answer. Type says which field holds the answer: predicate uses Predicate and
// Probability, choice uses Choice, score uses Score.
type DecisionAnswer struct {
	// Type is "predicate", "choice" or "score".
	Type string
	// Predicate is the yes or no answer for a predicate question.
	Predicate bool
	// Probability is the model's confidence in Predicate, between 0 and 1.
	Probability float64
	// Choice is the chosen option for a choice question.
	Choice string
	// Score is the integer level for a score question.
	Score float64
	// Probabilities maps each option of a choice or score answer to its probability, when the model reported them.
	Probabilities map[string]float64
	// Confidence is the model's overall confidence in a choice or score answer, when it reported one.
	Confidence *float64
}

// decisionWire is the response body of the decision route.
type decisionWire struct {
	ID      string                `json:"id"`
	Model   string                `json:"model"`
	Answers map[string]answerWire `json:"answers"`
	Usage   *decisionUsageWire    `json:"usage"`
}

// decisionUsageWire carries the billed token counts.
type decisionUsageWire struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

// answerWire is one answer before its value is checked against its type.
type answerWire struct {
	Type          string             `json:"type"`
	Value         json.RawMessage    `json:"value"`
	Probability   *float64           `json:"probability"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    *float64           `json:"confidence"`
}

// result checks the response shape the way the desktop app does before trusting the answers, and converts it.
func (w decisionWire) result() (DecisionResult, error) {
	if w.ID == "" || w.Model == "" || w.Answers == nil || w.Usage == nil {
		return DecisionResult{}, unexpectedDecisionShape("missing id, model, answers or usage")
	}
	answers := make(map[string]DecisionAnswer, len(w.Answers))
	for id, raw := range w.Answers {
		answer, err := raw.answer()
		if err != nil {
			return DecisionResult{}, unexpectedDecisionShape(fmt.Sprintf("answer %q: %v", id, err))
		}
		answers[id] = answer
	}
	return DecisionResult{
		ID:           w.ID,
		Model:        w.Model,
		Answers:      answers,
		InputTokens:  w.Usage.InputTokens,
		OutputTokens: w.Usage.OutputTokens,
	}, nil
}

// answer decodes one answer, checking that its value has the type it names.
func (w answerWire) answer() (DecisionAnswer, error) {
	switch w.Type {
	case "predicate":
		var value bool
		if err := json.Unmarshal(w.Value, &value); err != nil {
			return DecisionAnswer{}, fmt.Errorf("predicate value is not a boolean")
		}
		if w.Probability == nil {
			return DecisionAnswer{}, fmt.Errorf("predicate has no probability")
		}
		return DecisionAnswer{Type: w.Type, Predicate: value, Probability: *w.Probability}, nil
	case "choice":
		var value string
		if err := json.Unmarshal(w.Value, &value); err != nil {
			return DecisionAnswer{}, fmt.Errorf("choice value is not a string")
		}
		return DecisionAnswer{Type: w.Type, Choice: value, Probabilities: w.Probabilities, Confidence: w.Confidence}, nil
	case "score":
		var value float64
		if err := json.Unmarshal(w.Value, &value); err != nil {
			return DecisionAnswer{}, fmt.Errorf("score value is not a number")
		}
		return DecisionAnswer{Type: w.Type, Score: value, Probabilities: w.Probabilities, Confidence: w.Confidence}, nil
	default:
		return DecisionAnswer{}, fmt.Errorf("unknown answer type %q", w.Type)
	}
}

// unexpectedDecisionShape is the error for a decision response the client cannot trust.
func unexpectedDecisionShape(detail string) *Error {
	return &Error{Kind: KindBadResponse, Status: 200, Message: "decision response has an unexpected shape: " + detail}
}
