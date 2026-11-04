package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/charsearch"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// fixedToken is the signed-in account's token for the fake backend.
type fixedToken struct{}

func (fixedToken) Get() (string, error) { return "test-token", nil }

// decisionsBackend returns a real backend client over a fake HTTP port whose decisions route answers the way
// pleadingModel does, read from the wire request. The port records every request the picker sends.
func decisionsBackend(t *testing.T) (*backend.Client, *fake.HTTP) {
	t.Helper()
	clock := fake.NewClock()
	logger, err := logx.New(nil, clock, "", logx.Options{})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	env, err := backend.NewEnvironment(backend.EnvironmentProduction, "", false)
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}
	port := fake.NewHTTP()
	port.Handle(http.HandlerFunc(serveDecisions))
	client, err := backend.New(port, logger, env, fixedToken{}, backend.WithClock(clock), backend.WithJitter(func(d time.Duration) time.Duration { return d }))
	if err != nil {
		t.Fatalf("backend.New: %v", err)
	}
	return client, port
}

// serveDecisions answers POST /api/v1/decisions with the wire shape of the decision route.
func serveDecisions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Questions []backend.DecisionQuestion `json:"questions"`
	}
	if r.Method != http.MethodPost || r.URL.Path != "/api/v1/decisions" || json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "unexpected request", http.StatusBadRequest)
		return
	}
	answers := make(map[string]any, len(req.Questions))
	for _, q := range req.Questions {
		answers[q.ID] = pleadingWire(q)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": "dec_1", "model": "fixture", "answers": answers,
		"usage": map[string]int{"inputTokens": 10, "outputTokens": 2},
	})
}

// pleadingWire is the wire answer to one question: the same answers as pleadingModel gives.
func pleadingWire(q backend.DecisionQuestion) map[string]any {
	switch q.Type {
	case "predicate":
		p := pleadingLikelihood(q.Question)
		return map[string]any{"type": "predicate", "value": p >= 0.5, "probability": p}
	default:
		return map[string]any{"type": "choice", "value": "none of these", "probabilities": pleadingChoices(q.Options)}
	}
}

// TestPickerAsksTheDecisionsRouteAndShowsItsMatch checks the second batch of a real client: the request is a POST
// to the decisions route with the search text as input, and the answer's match follows the catalog's own rows.
func TestPickerAsksTheDecisionsRouteAndShowsItsMatch(t *testing.T) {
	client, port := decisionsBackend(t)
	p := goldenPalette(t)
	zero := time.Duration(0)
	p.characterSearch = &characterSearch{decide: func() charsearch.Decider { return client }, pause: &zero}
	openEmojiPicker(p)
	goldenQuery(p, "crying")

	titles := rowTitles(p)
	if len(titles) < 2 || titles[len(titles)-1] != "Pleading Face" {
		t.Fatalf("rows = %v, want the backend's pleading face last", titles)
	}
	requests := port.Requests()
	if len(requests) < 3 {
		t.Fatalf("requests = %d, want the three decision steps", len(requests))
	}
	if requests[0].Method != http.MethodPost || !strings.HasSuffix(requests[0].URL, "/api/v1/decisions") {
		t.Fatalf("request = %s %s, want a POST to the decisions route", requests[0].Method, requests[0].URL)
	}
	if !strings.Contains(string(requests[0].Body), `"input":"Search text: \"crying\""`) {
		t.Fatalf("body = %s, want the search text as the decision input", requests[0].Body)
	}
}

// TestPickerStopsAskingAfterThirtyDecisionsAMinute types distinct texts the catalog cannot answer. Each costs three
// decisions, so the eleventh text is refused by the rate limit and shows no match, while the first ten show theirs.
func TestPickerStopsAskingAfterThirtyDecisionsAMinute(t *testing.T) {
	client, _ := decisionsBackend(t)
	p := goldenPalette(t)
	zero := time.Duration(0)
	p.characterSearch = &characterSearch{decide: func() charsearch.Decider { return client }, pause: &zero}
	openEmojiPicker(p)
	for i := range 10 {
		goldenQuery(p, fmt.Sprintf("zzqx %d", i))
		if titles := rowTitles(p); len(titles) != 1 || titles[0] != "Pleading Face" {
			t.Fatalf("search %d rows = %v, want the pleading face", i, titles)
		}
	}
	goldenQuery(p, "zzqx 10")
	if titles := rowTitles(p); len(titles) != 0 {
		t.Fatalf("rate-limited rows = %v, want none", titles)
	}
}
