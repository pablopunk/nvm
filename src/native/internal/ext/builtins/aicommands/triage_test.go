package aicommands

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// triageContext is the context the triage calls, over fakes for the account, the selection, the indicator and the
// capabilities. Every other accessor is nil, so a test that reaches one fails loudly.
type triageContext struct {
	api.Context
	ai         *fakeTriageAI
	selection  *fakeSelection
	indicators *fakeIndicators
	caps       map[string]bool
}

func (c triageContext) AI() api.AI                 { return c.ai }
func (c triageContext) Desktop() api.Desktop       { return fakeDesktop{selection: c.selection} }
func (c triageContext) Indicators() api.Indicators { return c.indicators }
func (c triageContext) System() api.System         { return fakeSystem{caps: c.caps} }

type fakeDesktop struct {
	api.Desktop
	selection *fakeSelection
}

func (d fakeDesktop) Selection() api.Selection { return d.selection }

type fakeSelection struct {
	api.Selection
	text  string
	found bool
	reads int
}

func (s *fakeSelection) Text() (string, bool, error) {
	s.reads++
	return s.text, s.found, nil
}

type fakeSystem struct {
	api.System
	caps map[string]bool
}

func (s fakeSystem) HasCapability(name string) bool { return s.caps[name] }

type fakeTriageAI struct {
	api.AI
	available bool
	answers   map[string]api.DecisionAnswer
	requests  []api.DecisionRequest
}

func (a *fakeTriageAI) IsAvailable() (bool, error) { return a.available, nil }

func (a *fakeTriageAI) Chats() api.AIChats { return nil }

func (a *fakeTriageAI) Decide(request api.DecisionRequest) (api.DecisionResult, error) {
	a.requests = append(a.requests, request)
	return api.DecisionResult{Answers: a.answers}, nil
}

type fakeIndicators struct {
	api.Indicators
	shown   []api.IndicatorInput
	updates []api.IndicatorInput
}

func (i *fakeIndicators) Show(input api.IndicatorInput) error {
	i.shown = append(i.shown, input)
	return nil
}

func (i *fakeIndicators) Update(input api.IndicatorInput) error {
	i.updates = append(i.updates, input)
	return nil
}

func newTriageContext(text string, found, available bool) triageContext {
	return triageContext{
		ai:         &fakeTriageAI{available: available, answers: triageAnswers()},
		selection:  &fakeSelection{text: text, found: found},
		indicators: &fakeIndicators{},
		caps:       map[string]bool{selectionCopyCapability: true},
	}
}

func triageAnswers() map[string]api.DecisionAnswer {
	return map[string]api.DecisionAnswer{
		"needs_reply": {Type: api.DecisionPredicate, Flag: true, Probability: 0.9},
		"category":    {Type: api.DecisionChoice, Choice: "bug report"},
		"urgency":     {Type: api.DecisionScore, Score: 4},
	}
}

// TestTriageShowsTheSummaryOfTheAnswersForTheSelection checks the selection goes to the decision route as a JSON string
// with the three questions, and the indicator shows the summary as a success for the result's time.
func TestTriageShowsTheSummaryOfTheAnswersForTheSelection(t *testing.T) {
	ctx := newTriageContext("The export crashes on save, please fix", true, true)
	if _, err := (Extension{}).RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: TriageCommandID}}); err != nil {
		t.Fatalf("triage failed: %v", err)
	}
	if len(ctx.ai.requests) != 1 {
		t.Fatalf("decision requests = %d, want one", len(ctx.ai.requests))
	}
	request := ctx.ai.requests[0]
	var input string
	if err := json.Unmarshal(request.Input, &input); err != nil || input != "The export crashes on save, please fix" {
		t.Fatalf("decision input = %s (%v), want the selected text as a JSON string", request.Input, err)
	}
	if got := questionIDs(request.Questions); !slices.Equal(got, []string{"needs_reply", "category", "urgency"}) {
		t.Fatalf("questions = %v, want the three triage questions", got)
	}
	last := ctx.indicators.updates[len(ctx.indicators.updates)-1]
	if last.Status != api.IndicatorSuccess || last.Subtitle != "bug report · urgency 4/5 · needs reply" || last.DurationMs != triageDismissMs {
		t.Fatalf("final indicator = %+v, want the summary as a success for %d ms", last, triageDismissMs)
	}
}

// TestTriageRefusesASignedOutAccountBeforeReadingTheSelection checks a triage without an account explains itself and
// leaves the selection alone.
func TestTriageRefusesASignedOutAccountBeforeReadingTheSelection(t *testing.T) {
	ctx := newTriageContext("text", true, false)
	if _, err := (Extension{}).RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: TriageCommandID}}); err == nil {
		t.Fatal("a signed-out triage succeeded")
	}
	if ctx.selection.reads != 0 {
		t.Fatalf("the selection was read %d times before the account check", ctx.selection.reads)
	}
	if last := ctx.indicators.updates[len(ctx.indicators.updates)-1]; last.Status != api.IndicatorError || last.Subtitle != errNotSignedIn.Error() {
		t.Fatalf("indicator = %+v, want the sign-in copy as an error", last)
	}
}

// TestTriageWithoutASelectionSaysSo checks an empty selection fails with the copy that asks the user to select again.
func TestTriageWithoutASelectionSaysSo(t *testing.T) {
	ctx := newTriageContext("   ", true, true)
	if _, err := (Extension{}).RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: TriageCommandID}}); err == nil {
		t.Fatal("a triage of an empty selection succeeded")
	}
	if len(ctx.ai.requests) != 0 {
		t.Fatal("an empty selection reached the decision route")
	}
}

// TestTriageSummaryLeavesOutWhatIsMissing checks a summary names the answers it has and says so when it has none.
func TestTriageSummaryLeavesOutWhatIsMissing(t *testing.T) {
	if got := triageSummary(map[string]api.DecisionAnswer{}); got != "No triage result" {
		t.Fatalf("empty summary = %q", got)
	}
	answers := map[string]api.DecisionAnswer{"needs_reply": {Type: api.DecisionPredicate}}
	if got := triageSummary(answers); got != "no reply needed" {
		t.Fatalf("summary = %q, want the reply answer alone", got)
	}
}

// TestTriageIsListedOnlyWhenTheOSCanCopyTheSelection checks the triage row exists beside the fix only with the selection
// capability, so the palette never offers a triage that cannot run.
func TestTriageIsListedOnlyWhenTheOSCanCopyTheSelection(t *testing.T) {
	without := triageContext{caps: map[string]bool{}}
	commands, err := (Extension{}).Commands(without)
	if err != nil || len(commands) != 0 {
		t.Fatalf("commands without selection copy = %v (%v), want none", commandIDs(commands), err)
	}
	with := triageContext{caps: map[string]bool{selectionCopyCapability: true}}
	commands, err = (Extension{}).Commands(with)
	if err != nil || !slices.Equal(commandIDs(commands), []string{FixCommandID, TriageCommandID}) {
		t.Fatalf("commands = %v (%v), want the fix then the triage", commandIDs(commands), err)
	}
}

func questionIDs(questions []api.DecisionQuestion) []string {
	ids := make([]string, 0, len(questions))
	for _, question := range questions {
		ids = append(ids, question.ID)
	}
	return ids
}

func commandIDs(commands []api.Command) []string {
	ids := make([]string, 0, len(commands))
	for _, command := range commands {
		ids = append(ids, command.ID)
	}
	return ids
}
