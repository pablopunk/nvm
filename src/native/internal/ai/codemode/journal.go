package codemode

import (
	"fmt"
	"strings"
)

// outcome is how the gate dealt with one call.
type outcome string

const (
	outcomeRan      outcome = "ran"
	outcomeFailed   outcome = "failed"
	outcomeDeclined outcome = "declined"
	// outcomeUnanswered is a question the user did not answer in time. It is a refusal: nothing runs without an answer.
	outcomeUnanswered outcome = "unanswered"
	outcomeRefused    outcome = "refused"
	outcomeCancelled  outcome = "cancelled"
)

// entry is one gated call as the journal keeps it. Only the gate writes entries, from what it decided and what the host
// answered, so the model reads what happened and cannot invent an effect.
type entry struct {
	class   Class
	outcome outcome
	// line says what happened in the past tense, for the model and the row of the call.
	line string
}

// journalText is what the model reads after the program's value: every call that changed something or was refused, and
// a count of the reads.
func journalText(entries []entry) string {
	if len(entries) == 0 {
		return "No Nevermind calls were made."
	}
	var b strings.Builder
	reads := 0
	for _, e := range entries {
		if e.class == ClassRead && e.outcome == outcomeRan {
			reads++
			continue
		}
		fmt.Fprintf(&b, "- %s\n", e.line)
	}
	if reads > 0 {
		fmt.Fprintf(&b, "- %d read-only %s.\n", reads, plural(reads, "call", "calls"))
	}
	return "What happened (written by the host, not by the program):\n" + b.String()
}

// summary is the one line the program's row shows: how many changes ran and how many calls did not.
func summary(entries []entry) string {
	var changes, declined, refused, failed int
	for _, e := range entries {
		switch {
		case e.outcome == outcomeRan && e.class.changesData():
			changes++
		case e.outcome == outcomeDeclined:
			declined++
		case e.outcome == outcomeRefused || e.outcome == outcomeUnanswered:
			refused++
		case e.outcome == outcomeFailed:
			failed++
		}
	}
	var parts []string
	if changes > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", changes, plural(changes, "change", "changes")))
	}
	if declined > 0 {
		parts = append(parts, fmt.Sprintf("%d declined by you", declined))
	}
	if refused > 0 {
		parts = append(parts, fmt.Sprintf("%d refused", refused))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if len(parts) == 0 {
		return "Ran"
	}
	return strings.Join(parts, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
