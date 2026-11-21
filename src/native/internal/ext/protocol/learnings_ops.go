package protocol

import "github.com/pablopunk/nvm/native/internal/ext/api"

// Operations of the learnings service, which only built-in extensions may call.
const (
	OpLearningsList   = "list"
	OpLearningsEdit   = "edit"
	OpLearningsDelete = "delete"
)

// LearningsChangeArgs names the rule an edit or delete targets and the revisions the caller read. Text is the new
// text of an edit.
type LearningsChangeArgs struct {
	ListRevision uint64 `json:"listRevision"`
	RuleID       string `json:"ruleId"`
	RuleRevision uint64 `json:"ruleRevision"`
	Text         string `json:"text,omitempty"`
}

// LearningsOutcomeAnswer is the answer to edit and delete.
type LearningsOutcomeAnswer struct {
	Outcome api.LearningOutcome `json:"outcome"`
}
