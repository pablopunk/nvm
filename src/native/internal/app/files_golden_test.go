package app

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/extai"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	filesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/files"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// The Files goldens render the built-in files extension's views at the palette's size.
var (
	meaningGoldens = golden.Options{Group: "meaningfiles", Width: width, Height: height}
	filesGoldens   = golden.Options{Group: "files", Width: width, Height: height}
)

// beachFiles is the index the goldens search: two names contain "beach", and a sunset picture is there for the decision
// to find.
func beachFiles(home string) []string {
	return []string{
		filepath.Join(home, "Pictures", "beach-2024.jpg"),
		filepath.Join(home, "Documents", "beach-notes.txt"),
		filepath.Join(home, "Pictures", "sunset.png"),
	}
}

// fixedAccount is the account state the host answers the built-in with.
type fixedAccount bool

func (a fixedAccount) SignedIn() bool { return bool(a) }

// filesPalette is the palette the Files goldens run on: the built-in files extension over beachFiles, the account signed
// in or out, and the decision answered by client. A nil client leaves the decision without a backend.
func filesPalette(t *testing.T, signedIn bool, client *backend.Client) (*App, *manager.Manager) {
	t.Helper()
	p := goldenPalette(t)
	home := goldenHome(t)
	p.files = newTestFiles(t, home, beachFiles(home))
	ai := extai.New(extai.Deps{
		Client: func() *backend.Client { return client }, Connected: func() bool { return signedIn },
		Clock: fake.NewClock(), Clipboard: fake.NewClipboard(), Files: fake.NewFS(), Home: home,
	})
	m := newManagerWithFakes(t, managerFakes{index: fileIndexPort{p: p}, ai: ai, account: fixedAccount(signedIn)}, []api.Extension{filesbuiltin.New()}, nil, nil, nil)
	p.exts.mgr.Store(m)
	return p, m
}

// openFilesView runs the built-in's command that opens a view and shows the view on p. The view's searches run on the
// test's clock and its UI callbacks are posted for settleSearchChange.
func openFilesView(t *testing.T, p *App, m *manager.Manager, command string) (*extview.Session, *fake.Clock, chan func()) {
	t.Helper()
	clock := fake.NewClock()
	posted := make(chan func(), 4)
	deps := syncViewDeps(m)
	deps.Clock = clock
	deps.OnUI = func(fn func()) { posted <- fn }
	result, failure, err := m.Run(context.Background(), filesbuiltin.ID, api.Invocation{Ref: api.ActionRef{ID: command}})
	if err != nil || failure != nil || result.View == nil {
		t.Fatalf("open %s: err %v, failure %+v", command, err, failure)
	}
	session := extview.NewSession(deps, filesbuiltin.ID, *result.View)
	goldenOpen(p, session.Open(refreshingHost{App: p}))
	return session, clock, posted
}

// typeAndRest types a query into the open view and rests, so the view's search answers and its rows show.
func typeAndRest(t *testing.T, p *App, clock *fake.Clock, posted chan func(), query string) {
	t.Helper()
	goldenQuery(p, query)
	clock.Advance(350 * time.Millisecond)
	settleSearchChange(t, posted)
}

// askMeaning presses the ask row for query: the built-in decides, and the view takes its rows.
func askMeaning(t *testing.T, p *App, m *manager.Manager, session *extview.Session, query string) {
	t.Helper()
	ask := api.ActionRef{ID: "meaning-ask", Args: map[string]string{"query": query}}
	result, failure, err := m.Run(context.Background(), filesbuiltin.ID, api.Invocation{Ref: ask})
	if err != nil || failure != nil || result.Patch == nil {
		t.Fatalf("ask for %q: err %v, failure %+v", query, err, failure)
	}
	session.Apply(*result.Patch)
	refreshingHost{App: p}.Refresh()
}

// TestGoldenFindFilesByMeaningPrompt is the view before any query: the recent files, and no ask row.
func TestGoldenFindFilesByMeaningPrompt(t *testing.T) {
	p, m := filesPalette(t, true, nil)
	openFilesView(t, p, m, "find-files-by-meaning")
	golden.AssertMatrix(t, "empty", p.View, meaningGoldens)
}

// TestGoldenFindFilesByMeaningAskRow is the name matches with the ask row above them, before the decision runs.
func TestGoldenFindFilesByMeaningAskRow(t *testing.T) {
	p, m := filesPalette(t, true, nil)
	_, clock, posted := openFilesView(t, p, m, "find-files-by-meaning")
	typeAndRest(t, p, clock, posted, "beach")
	if titles := rowLabelsOf(p.Results()); !slices.Equal(titles, []string{"Find more files with AI", "beach-2024.jpg", "beach-notes.txt"}) {
		t.Fatalf("rows = %v, want the ask row above the two name matches", titles)
	}
	golden.AssertMatrix(t, "ask", p.View, meaningGoldens)
}

// TestGoldenFindFilesByMeaningSignedOut has no ask row for a signed-out account.
func TestGoldenFindFilesByMeaningSignedOut(t *testing.T) {
	p, m := filesPalette(t, false, nil)
	_, clock, posted := openFilesView(t, p, m, "find-files-by-meaning")
	typeAndRest(t, p, clock, posted, "beach")
	golden.AssertMatrix(t, "signed-out", p.View, meaningGoldens)
}

// TestGoldenFindFilesByMeaningMatches shows the decision's match after the name matches.
func TestGoldenFindFilesByMeaningMatches(t *testing.T) {
	client, _ := decisionsBackendServing(t, serveSunsetDecisions)
	p, m := filesPalette(t, true, client)
	session, clock, posted := openFilesView(t, p, m, "find-files-by-meaning")
	typeAndRest(t, p, clock, posted, "holiday")
	askMeaning(t, p, m, session, "holiday")
	if titles := rowLabelsOf(p.Results()); !slices.Equal(titles, []string{"sunset.png"}) {
		t.Fatalf("rows = %v, want the decision's sunset picture", titles)
	}
	golden.AssertMatrix(t, "matches", p.View, meaningGoldens)
}

// TestGoldenFindFilesByMeaningFailed explains a decision the backend refused.
func TestGoldenFindFilesByMeaningFailed(t *testing.T) {
	client, _ := decisionsBackendServing(t, serveCreditsRefusal)
	p, m := filesPalette(t, true, client)
	session, clock, posted := openFilesView(t, p, m, "find-files-by-meaning")
	typeAndRest(t, p, clock, posted, "holiday")
	askMeaning(t, p, m, session, "holiday")
	if titles := rowLabelsOf(p.Results()); len(titles) != 1 || !strings.HasPrefix(titles[0], "Could not search files by meaning: ") {
		t.Fatalf("rows = %v, want the failure that says why", titles)
	}
	golden.AssertMatrix(t, "failed", p.View, meaningGoldens)
}

// TestGoldenSearchFilesFooter is the Search Files view as it opens: the recent files, the kind key and the footer count.
func TestGoldenSearchFilesFooter(t *testing.T) {
	p, m := filesPalette(t, true, nil)
	openFilesView(t, p, m, "search-files")
	golden.AssertMatrix(t, "search-files", p.View, filesGoldens)
}

// serveSunsetDecisions answers the decision as the model would for "holiday": the sunset picture is the one file it
// finds relevant. It reads the names the extension sent in the decision's input.
func serveSunsetDecisions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Input     string                     `json:"input"`
		Questions []backend.DecisionQuestion `json:"questions"`
	}
	if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "unexpected request", http.StatusBadRequest)
		return
	}
	var input struct {
		Files []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"files"`
	}
	_ = json.Unmarshal([]byte(req.Input), &input)
	names := make(map[string]string, len(input.Files))
	for _, file := range input.Files {
		names[file.ID] = file.Name
	}
	answers := make(map[string]any, len(req.Questions))
	for _, q := range req.Questions {
		probability := 0.1
		if strings.Contains(names[q.ID], "sunset") {
			probability = 0.9
		}
		answers[q.ID] = map[string]any{"type": "predicate", "value": probability >= 0.5, "probability": probability}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": "dec_1", "model": "fixture", "answers": answers,
		"usage": map[string]int{"inputTokens": 10, "outputTokens": 2},
	})
}

// serveCreditsRefusal refuses the decision for lack of credits, as the backend does.
func serveCreditsRefusal(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPaymentRequired)
	_, _ = w.Write([]byte(`{"error":{"type":"insufficient_credits","message":"not enough credits."}}`))
}
