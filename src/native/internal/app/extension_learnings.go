package app

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/ai/learning"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// errLearningsUnavailable answers the rules list while the learning store has not opened (or never will: read-only
// profile, missing key).
var errLearningsUnavailable = errors.New("AI learnings are unavailable")

// extensionLearnings serves the stored builder rules to the built-in AI Learnings extension. Every change goes through
// the learning service, so the revision guards, the deletion fingerprints and the invalidation of earlier reviews hold.
type extensionLearnings struct {
	p *App
}

func (l extensionLearnings) service() (*learning.Service, error) {
	rt := l.p.builderLearning.Load()
	if rt == nil {
		return nil, errLearningsUnavailable
	}
	return rt.service, nil
}

// Rules lists the committed rules with the revision of the list.
func (l extensionLearnings) Rules() (api.LearningList, error) {
	service, err := l.service()
	if err != nil {
		return api.LearningList{}, err
	}
	doc, err := service.Snapshot()
	if err != nil {
		return api.LearningList{}, err
	}
	list := api.LearningList{Revision: doc.Revision, Rules: make([]api.LearningRule, 0, len(doc.Rules))}
	for _, rule := range doc.Rules {
		list.Rules = append(list.Rules, api.LearningRule{ID: rule.ID, Kind: rule.Kind, Text: rule.Text, AppliesWhen: rule.AppliesWhen, Revision: rule.Revision})
	}
	return list, nil
}

// Edit replaces the text of a rule, keeping its kind and condition. A list or rule that moved on is stale.
func (l extensionLearnings) Edit(listRevision uint64, ruleID string, ruleRevision uint64, text string) (api.LearningOutcome, error) {
	service, err := l.service()
	if err != nil {
		return "", err
	}
	doc, err := service.Snapshot()
	if err != nil {
		return "", err
	}
	if doc.Revision != listRevision {
		return api.LearningStale, nil
	}
	for _, rule := range doc.Rules {
		if rule.ID == ruleID {
			rule.Text, rule.Revision = text, ruleRevision
			return learningOutcome(service.Edit(listRevision, rule))
		}
	}
	return api.LearningStale, nil
}

// Delete removes a rule and suppresses it from reviews that started before.
func (l extensionLearnings) Delete(listRevision uint64, ruleID string, ruleRevision uint64) (api.LearningOutcome, error) {
	service, err := l.service()
	if err != nil {
		return "", err
	}
	return learningOutcome(service.Delete(listRevision, ruleID, ruleRevision))
}

// learningOutcome turns the store's refusals into outcomes the extension shows; other errors stay errors.
func learningOutcome(err error) (api.LearningOutcome, error) {
	switch {
	case err == nil:
		return api.LearningSaved, nil
	case errors.Is(err, learning.ErrConflict):
		return api.LearningStale, nil
	case errors.Is(err, learning.ErrInvalid):
		return api.LearningInvalid, nil
	}
	return "", err
}
