package app

import (
	"encoding/json"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"net/http"
	"testing"
	"time"
)

// fixedToken is the signed-in account's token for the fake backend.
type fixedToken struct{}

func (fixedToken) Get() (string, error) { return "test-token", nil }

// decisionsBackend returns a real backend client over a fake HTTP port whose decisions route answers the way
// pleadingModel does, read from the wire request. The port records every request the picker sends.
func decisionsBackend(t *testing.T) (*backend.Client, *fake.HTTP) {
	t.Helper()
	return decisionsBackendServing(t, serveDecisions)
}

// decisionsBackendServing is decisionsBackend with the handler of the decisions route given.
func decisionsBackendServing(t *testing.T, serve http.HandlerFunc) (*backend.Client, *fake.HTTP) {
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
	port.Handle(serve)
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
