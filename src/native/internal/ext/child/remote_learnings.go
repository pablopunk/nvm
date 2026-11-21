package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteLearnings is the AI learnings of a built-in extension: the host keeps the rules and checks every revision.
type remoteLearnings struct {
	link *hostLink
}

func (l remoteLearnings) List() (api.LearningList, error) {
	var list api.LearningList
	err := l.link.Call(protocol.ServiceLearnings, protocol.OpLearningsList, nil, &list)
	return list, err
}

func (l remoteLearnings) Edit(listRevision uint64, ruleID string, ruleRevision uint64, text string) (api.LearningOutcome, error) {
	return l.change(protocol.OpLearningsEdit, protocol.LearningsChangeArgs{ListRevision: listRevision, RuleID: ruleID, RuleRevision: ruleRevision, Text: text})
}

func (l remoteLearnings) Delete(listRevision uint64, ruleID string, ruleRevision uint64) (api.LearningOutcome, error) {
	return l.change(protocol.OpLearningsDelete, protocol.LearningsChangeArgs{ListRevision: listRevision, RuleID: ruleID, RuleRevision: ruleRevision})
}

func (l remoteLearnings) change(op string, args protocol.LearningsChangeArgs) (api.LearningOutcome, error) {
	var answer protocol.LearningsOutcomeAnswer
	err := l.link.Call(protocol.ServiceLearnings, op, args, &answer)
	return answer.Outcome, err
}
