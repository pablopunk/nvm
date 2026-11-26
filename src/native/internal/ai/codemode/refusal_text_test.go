package codemode

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// A refusal travels from the host's editor to the program's child and back to the model. Every service's refusal names
// its cause once: the child keeps the host's message instead of wrapping the sentinel's text around it again.
func TestARefusalReachesTheModelWithItsCauseNamedOnce(t *testing.T) {
	cases := map[string]struct {
		fixture func(*testing.T) *fixture
		body    string
		reason  string
	}{
		"shortcut taken (#324)": {func(t *testing.T) *fixture { return newFixture(t, withShortcuts(newFakeShortcutTable())) },
			`_, err := ctx.Shortcuts().Set("notes", "Command+Alt+V")` + "\nreturn nil, err", "already used"},
		"alias taken (#326)": {func(t *testing.T) *fixture { f, _ := aliasFixture(t); return f },
			`return nil, ctx.Shortcuts().AddAlias("clipboard", "jot")`, "already an alias of Search Notes"},
		"job already on (#326)": {func(t *testing.T) *fixture { f, _ := jobsFixture(t); return f },
			`return nil, ctx.Jobs().SetEnabled("host.index", true)`, "is already on"},
		"setting value not declared (#327)": {func(t *testing.T) *fixture { f, _ := preferenceFixture(t); return f },
			`_, err := ctx.Preferences().Set("nevermind.settings", "clipboardLimit", "999")` + "\nreturn nil, err", "accepts only"},
		"extension already on (#328)": {func(t *testing.T) *fixture { f, _ := extensionsFixture(t); return f },
			`return nil, ctx.Extensions().OpenReview("acme.clock")`, "is already on"},
		"tool call the extension refuses (#330)": {func(t *testing.T) *fixture { f, _ := toolsFixture(t); return f },
			`return ctx.Tools().Call("nevermind.snippets", "delete-snippet", map[string]any{"id": "gone"})`, "no longer exists"},
	}
	doubled := api.ErrInvalidInput.Error() + ": " + api.ErrInvalidInput.Error()
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := c.fixture(t)
			f.steps.decide = allow
			result := f.run(c.body)
			t.Logf("model reads: %s", result.Text)
			if !result.Failed || !strings.Contains(result.Text, c.reason) {
				t.Fatalf("result = %+v; want a refusal that says %q", result, c.reason)
			}
			if strings.Contains(result.Text, doubled) {
				t.Fatalf("the refusal names its cause twice: %s", result.Text)
			}
			for _, row := range f.steps.snapshot() {
				if strings.Contains(row.outcome, doubled) {
					t.Fatalf("the transcript row names the cause twice: %+v", row)
				}
			}
		})
	}
}
