package api

// LearningRule is one durable rule the AI builder learned from the user's builder chats. Revision is its saved version:
// pass it back to Edit and Delete so a change made meanwhile is refused instead of overwritten.
type LearningRule struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Text        string `json:"text"`
	AppliesWhen string `json:"appliesWhen,omitempty"`
	Revision    uint64 `json:"revision"`
}

// LearningList is the stored rules and the revision of the whole list. Revision is what Edit and Delete expect as the
// list revision they were made against.
type LearningList struct {
	Revision uint64         `json:"revision"`
	Rules    []LearningRule `json:"rules"`
}

// LearningOutcome says what an Edit or Delete did. Nothing is written unless it is LearningSaved.
type LearningOutcome string

const (
	// LearningSaved: the change is stored.
	LearningSaved LearningOutcome = "saved"
	// LearningStale: the rules changed since the caller read them, perhaps by an automatic review. Nothing was
	// written; read the list again.
	LearningStale LearningOutcome = "stale"
	// LearningInvalid: the text is empty, too long or not allowed in a rule. Nothing was written.
	LearningInvalid LearningOutcome = "invalid"
)

// Learnings reads and changes the rules the AI builder learned. It is host-only: the built-in AI Learnings extension is
// its sole caller. Edits and deletes are guarded by revisions, and a deleted rule is not learned again by a review that
// started earlier.
type Learnings interface {
	List() (LearningList, error)
	// Edit replaces the text of the rule when the list and the rule are still at the given revisions.
	Edit(listRevision uint64, ruleID string, ruleRevision uint64, text string) (LearningOutcome, error)
	// Delete removes the rule when the list and the rule are still at the given revisions.
	Delete(listRevision uint64, ruleID string, ruleRevision uint64) (LearningOutcome, error)
}
