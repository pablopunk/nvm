package files

import (
	"encoding/json"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// The decision the model makes about the recent file names. A predicate answer whose probability reaches the match
// probability is a match, and the ranks of the matches keep the model's order.
const (
	matchProbability   = 0.5
	askRowScore        = 1000
	meaningScoreBase   = 10
	meaningAskID       = "meaning-ask"
	meaningFailureID   = "meaning-failure"
	meaningAskTitle    = "Find more files with AI"
	meaningAskSub      = "Send the names of your most recent files for analysis"
	meaningFindTitle   = "Find Files"
	meaningTitle       = "Find Files by Meaning"
	meaningPlaceholder = "Describe the file you want"
	meaningEmptySub    = "Type what the file is about. Your recent file names can be checked with AI."
	meaningFailure     = "Could not search files by meaning."
	questionFormat     = "Is %s relevant to the search query?"
)

// decisionInput is the document the model reads: the query and the candidates' names by ID.
type decisionInput struct {
	Query string         `json:"query"`
	Files []decisionFile `json:"files"`
}

type decisionFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// candidateID is the key of the candidate at index i, in the questions and the answers.
func candidateID(i int) string {
	return fmt.Sprintf("file_%d", i)
}

// judge asks the model which candidates are relevant to query and returns their indexes, in the order given.
func judge(ctx api.Context, query string, candidates []api.FileRef) ([]int, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	input := decisionInput{Query: query, Files: make([]decisionFile, 0, len(candidates))}
	questions := make([]api.DecisionQuestion, 0, len(candidates))
	for i, file := range candidates {
		id := candidateID(i)
		input.Files = append(input.Files, decisionFile{ID: id, Name: file.Name})
		questions = append(questions, api.DecisionQuestion{ID: id, Type: api.DecisionPredicate, Question: fmt.Sprintf(questionFormat, id)})
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	shared, err := json.Marshal(string(encoded))
	if err != nil {
		return nil, err
	}
	result, err := ctx.AI().Decide(api.DecisionRequest{Input: shared, Questions: questions})
	if err != nil {
		return nil, err
	}
	return matchedIndexes(result, len(candidates)), nil
}

// matchedIndexes returns the candidates whose predicate answer reaches the match probability.
func matchedIndexes(result api.DecisionResult, count int) []int {
	var matches []int
	for i := range count {
		answer, ok := result.Answers[candidateID(i)]
		if ok && answer.Type == api.DecisionPredicate && answer.Probability >= matchProbability {
			matches = append(matches, i)
		}
	}
	return matches
}

// meaningRows are the view rows of Find Files by Meaning for a query: the name matches, then either the ask row, the
// decision's matches, or the failure the decision left. The decision runs only after the user asks for this query.
func (e Extension) meaningRows(ctx api.Context, query string) []api.Item {
	items := withThumbnails(ctx, e.rowsFor(ctx, query, "", meaningLocal))
	if query == "" {
		return items
	}
	if !e.state.askedFor(query) {
		if ctx.Account().SignedIn() {
			items = append(items, askRow(query))
		}
		return items
	}
	return append(items, e.state.decisionRows(query)...)
}

// askRow is the row that asks the model about the query.
func askRow(query string) api.Item {
	ask := api.Run(refMeaningAsk, meaningFindTitle, map[string]string{argQuery: query})
	return api.Item{
		ID: meaningAskID, Title: meaningAskTitle, Subtitle: meaningAskSub, Icon: "sparkles",
		Score: askRowScore, Primary: &ask, Section: search.KindFiles,
	}
}

// meaningSearchChanged answers a new text of Find Files by Meaning: the list is rebuilt for it.
func (e Extension) meaningSearchChanged(ctx api.Context, query string) api.Result {
	e.state.setQuery(query)
	items := e.meaningRows(ctx, query)
	return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: items}}
}

// askMeaning runs the decision for a query and lists its matches under the name matches. A failed decision lists a
// contextual failure row, and the ask does not repeat.
func (e Extension) askMeaning(ctx api.Context, query string) api.Result {
	rows, failure := e.decide(ctx, query)
	e.state.recordDecision(query, rows)
	items := withThumbnails(ctx, e.rowsFor(ctx, query, "", meaningLocal))
	items = append(items, rows...)
	if failure != nil {
		items = append(items, failureRow(failure))
	}
	return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: items}}
}

// decide finds the recent files the model judges relevant, scored by rank so the model's order holds.
func (e Extension) decide(ctx api.Context, query string) ([]api.Item, error) {
	candidates := recentFiles(ctx, meaningCandidates, "")
	matches, err := judge(ctx, query, candidates)
	if err != nil {
		return nil, err
	}
	rows := make([]api.Item, 0, len(matches))
	for rank, index := range matches {
		row := fileItem(ctx, candidates[index], float64(meaningScoreBase-rank))
		rows = append(rows, row)
	}
	return rows, nil
}

// failureRow is the contextual row a failed decision leaves in the list.
func failureRow(err error) api.Item {
	return api.Item{
		ID: meaningFailureID, Title: meaningFailure, Subtitle: err.Error(), Disabled: true, Contextual: true,
		Section: search.KindFiles,
	}
}

// meaningView is the Find Files by Meaning view: the name matches of the typed text, the ask row, and the decision's
// matches once the user asked for the text.
func (e Extension) meaningView(ctx api.Context) api.View {
	e.state.reset()
	items := e.meaningRows(ctx, "")
	view := api.ListView(meaningTitle, items...)
	view.Placeholder = meaningPlaceholder
	view.EmptyView = &api.EmptyState{Title: meaningPlaceholder, Subtitle: meaningEmptySub}
	view.OnSearchChange = &api.ActionRef{ID: refMeaningSearch}
	return view
}
