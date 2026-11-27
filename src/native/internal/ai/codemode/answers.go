package codemode

import (
	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// answerScope names exactly what one answer of the user covers: the operation, the scope the rule names, and the
// resource the call's arguments name, in canonical form. Another resource, or the same operation with other arguments,
// is a different question.
func answerScope(k key, scope string, call protocol.ServiceCall) string {
	return k.service + "." + k.op + "|" + scope + "|" + agent.CanonicalArguments(call.Args)
}

// earlierAnswer is the user's answer to the same question earlier in this run, when there was one the gate keeps.
func (g *gate) earlierAnswer(scope string) (allowed, answered bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	allowed, answered = g.answers[scope]
	return allowed, answered
}

// keepAnswer remembers a decline of any class, so the run never asks the same question twice after a no, and an allow
// only for a class whose one allow covers the repeats of the exact call.
func (g *gate) keepAnswer(class Class, scope string, answer outcome) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch {
	case answer == outcomeDeclined:
		g.answers[scope] = false
	case answer == outcomeRan && class.allowCoversRepeats():
		g.answers[scope] = true
	}
}
