package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// fixEvalPath is the case set of the correction eval: one JSON array of cases, each with an input and what the
// correction must do with it.
const fixEvalPath = "testdata/fixeval/cases.json"

// The outcomes a case may expect. fixed: the output is exactly the expected text. unchanged: the input comes back byte
// for byte. refused: no correction replaces the selection.
const (
	expectFixed     = "fixed"
	expectUnchanged = "unchanged"
	expectRefused   = "refused"
)

// evalCase is one row of the eval set. Model describes the scripted model that answers it in L1: explicit Replies
// for the first attempts, then the input with Substitutions applied, wrapped in Noise.
type evalCase struct {
	ID      string    `json:"id"`
	Group   string    `json:"group"`
	Input   string    `json:"input"`
	Expect  string    `json:"expect"`
	Output  string    `json:"output,omitempty"`
	Protect []string  `json:"protect,omitempty"`
	Model   evalModel `json:"model"`
}

// evalModel is the scripted model of one case.
type evalModel struct {
	Substitutions [][2]string `json:"substitutions,omitempty"`
	Noise         string      `json:"noise,omitempty"`
	Replies       []string    `json:"replies,omitempty"`
}

// loadFixEval reads the case set.
func loadFixEval(t *testing.T) []evalCase {
	t.Helper()
	raw, err := os.ReadFile(fixEvalPath)
	if err != nil {
		t.Fatalf("read the eval set: %v", err)
	}
	var cases []evalCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("parse the eval set: %v", err)
	}
	return cases
}

// scriptedAsker is the L1 model of a case: its replies first, then the substitutions applied to the piece it is asked
// about, with the case's noise around the answer.
func scriptedAsker(model evalModel) Asker {
	calls := 0
	return func(_, user string) (string, error) {
		index := calls
		calls++
		if index < len(model.Replies) {
			return model.Replies[index], nil
		}
		return noisy(applySubstitutions(pieceOf(user), model.Substitutions), model.Noise), nil
	}
}

// pieceOf returns the text between the tags of a user message.
func pieceOf(user string) string {
	start := strings.Index(user, "<text>") + len("<text>")
	end := strings.LastIndex(user, "</text>")
	return user[start:end]
}

// applySubstitutions replaces each from with its to, in order.
func applySubstitutions(text string, pairs [][2]string) string {
	for _, pair := range pairs {
		text = strings.ReplaceAll(text, pair[0], pair[1])
	}
	return text
}

// noisy wraps an answer the way a model sometimes does: in a fence, behind a preamble, in quotes or in the tags.
func noisy(answer, noise string) string {
	switch noise {
	case "fence":
		return "```\n" + answer + "\n```"
	case "preamble":
		return "Here is the corrected text:\n" + answer
	case "quotes":
		return "“" + answer + "”"
	case "tags":
		return "<text>" + answer + "</text>"
	}
	return answer
}

// scoreCase judges the correction of one case. It returns whether the case passed and, when it did not, why.
func scoreCase(c evalCase, got string, err error) (bool, string) {
	refusal := errors.Is(err, ErrCorrectionRefused) || errors.Is(err, ErrNoCorrection)
	switch c.Expect {
	case expectRefused:
		if refusal {
			return true, ""
		}
		return false, fmt.Sprintf("accepted %q (err %v)", got, err)
	case expectUnchanged:
		if err != nil {
			return false, fmt.Sprintf("error %v", err)
		}
		if got != c.Input {
			return false, fmt.Sprintf("changed to %q", got)
		}
	case expectFixed:
		if err != nil {
			return false, fmt.Sprintf("error %v", err)
		}
		if got != c.Output {
			return false, fmt.Sprintf("got %q, want %q", got, c.Output)
		}
	}
	for _, token := range c.Protect {
		if strings.Contains(c.Input, token) && !strings.Contains(got, token) {
			return false, fmt.Sprintf("lost protected %q", token)
		}
	}
	return true, ""
}

// TestFixEvalScoresTheScriptedCorrections runs every case through the correction pipeline against its scripted model.
// It prints a table per group and fails on any case that does not behave as its expectation says.
func TestFixEvalScoresTheScriptedCorrections(t *testing.T) {
	cases := loadFixEval(t)
	if len(cases) < 40 {
		t.Fatalf("the eval set has %d cases, want at least 40", len(cases))
	}
	type tally struct{ passed, total int }
	groups := map[string]*tally{}
	var order []string
	for _, c := range cases {
		got, err := CorrectText(c.Input, scriptedAsker(c.Model), nil)
		passed, reason := scoreCase(c, got, err)
		if groups[c.Group] == nil {
			groups[c.Group] = &tally{}
			order = append(order, c.Group)
		}
		groups[c.Group].total++
		if passed {
			groups[c.Group].passed++
			continue
		}
		t.Errorf("%s (%s, expects %s): %s", c.ID, c.Group, c.Expect, reason)
	}
	passed, total := 0, 0
	for _, group := range order {
		t.Logf("%-16s %2d/%2d", group, groups[group].passed, groups[group].total)
		passed += groups[group].passed
		total += groups[group].total
	}
	t.Logf("L1 fixeval: %d/%d cases pass", passed, total)
}
