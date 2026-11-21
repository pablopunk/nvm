package services

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// LearningsPort is what the learnings service reads and changes on the host: the stored builder rules. Changes are
// guarded by the revisions the caller read, and a refused change is an outcome, not an error.
type LearningsPort interface {
	Rules() (api.LearningList, error)
	Edit(listRevision uint64, ruleID string, ruleRevision uint64, text string) (api.LearningOutcome, error)
	Delete(listRevision uint64, ruleID string, ruleRevision uint64) (api.LearningOutcome, error)
}

// serveLearnings answers the learnings calls. Only built-in extensions may make them.
func (h *Handler) serveLearnings(call protocol.ServiceCall) (any, error) {
	if !h.manifest.IsBuiltIn() {
		return nil, unavailable("AI learnings are for built-in extensions only")
	}
	if h.deps.Learnings == nil {
		return nil, unavailable("AI learnings are not available in this build")
	}
	switch call.Op {
	case protocol.OpLearningsList:
		return h.deps.Learnings.Rules()
	case protocol.OpLearningsEdit, protocol.OpLearningsDelete:
		var args protocol.LearningsChangeArgs
		if err := decodeArgs(call.Args, &args); err != nil {
			return nil, err
		}
		if args.RuleID == "" {
			return nil, invalid("a learning rule id is required")
		}
		return h.changeLearning(call.Op, args)
	}
	return nil, invalid("learnings has no operation %q", call.Op)
}

func (h *Handler) changeLearning(op string, args protocol.LearningsChangeArgs) (protocol.LearningsOutcomeAnswer, error) {
	var outcome api.LearningOutcome
	var err error
	if op == protocol.OpLearningsEdit {
		outcome, err = h.deps.Learnings.Edit(args.ListRevision, args.RuleID, args.RuleRevision, args.Text)
	} else {
		outcome, err = h.deps.Learnings.Delete(args.ListRevision, args.RuleID, args.RuleRevision)
	}
	return protocol.LearningsOutcomeAnswer{Outcome: outcome}, err
}
