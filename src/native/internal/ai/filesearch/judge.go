package filesearch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// CandidateLimit is how many recent file names the decision model judges. It matches the Electron feature.
const CandidateLimit = 16

// MatchProbability is the probability from which a candidate counts as a match.
const MatchProbability = 0.5

// questionFormat is the predicate asked of every candidate; the ID in it is the candidate's key in the answers.
const questionFormat = "Is %s relevant to the search query?"

// Decider answers decision requests. The backend client satisfies it.
type Decider interface {
	Decisions(ctx context.Context, req backend.DecisionRequest) (backend.DecisionResult, error)
}

// decisionInput is the JSON document the model reads: the query and the candidates' names by ID.
type decisionInput struct {
	Query string           `json:"query"`
	Files []decisionFileIn `json:"files"`
}

// decisionFileIn is one candidate in the input: an ID the answers refer to, and the file's name.
type decisionFileIn struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// candidateID is the key of the candidate at index i in the questions and the answers.
func candidateID(i int) string {
	return fmt.Sprintf("file_%d", i)
}

// buildRequest builds the decision request that asks, for each name, whether it is relevant to query.
func buildRequest(query string, names []string) (backend.DecisionRequest, error) {
	input := decisionInput{Query: query, Files: make([]decisionFileIn, 0, len(names))}
	questions := make([]backend.DecisionQuestion, 0, len(names))
	for i, name := range names {
		id := candidateID(i)
		input.Files = append(input.Files, decisionFileIn{ID: id, Name: name})
		questions = append(questions, backend.DecisionQuestion{
			ID:       id,
			Type:     "predicate",
			Question: fmt.Sprintf(questionFormat, id),
		})
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return backend.DecisionRequest{}, err
	}
	return backend.DecisionRequest{Input: string(encoded), Questions: questions}, nil
}

// Judge asks the decision model which of names are relevant to query and returns their indexes, in the order given.
// An empty candidate list asks nothing. The model's answers are read as the desktop app reads them: a predicate answer
// with a probability of at least MatchProbability is a match, and an answer of another type or a missing answer is not.
func Judge(ctx context.Context, decider Decider, query string, names []string) ([]int, error) {
	if len(names) == 0 {
		return nil, nil
	}
	req, err := buildRequest(query, names)
	if err != nil {
		return nil, err
	}
	result, err := decider.Decisions(ctx, req)
	if err != nil {
		return nil, err
	}
	return matchedIndexes(result, len(names)), nil
}

// matchedIndexes returns the indexes of the candidates whose predicate answer reaches MatchProbability.
func matchedIndexes(result backend.DecisionResult, count int) []int {
	var matches []int
	for i := range count {
		answer, ok := result.Answers[candidateID(i)]
		if ok && answer.Type == "predicate" && answer.Probability >= MatchProbability {
			matches = append(matches, i)
		}
	}
	return matches
}
