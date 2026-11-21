package supervisor

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// paperlessServer answers like a Paperless-ngx API: a token check, one page of two matching documents with a next page,
// and no preview thumbnails.
func paperlessServer() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/thumb/"):
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Query().Get("page_size") == "1":
			_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[]}`))
		default:
			_, _ = w.Write([]byte(`{"count":45,"next":"https://paperless.example.com/api/documents/?page=2","results":[
				{"id":11,"title":"Acme Invoice March","content":"Invoice total 120 EUR","created":"2024-03-02","original_file_name":"acme-march.pdf"},
				{"id":12,"title":"Acme Invoice April","content":"Invoice total 95 EUR","created":"2024-04-02","original_file_name":"acme-april.pdf"}]}`))
		}
	})
}

// TestInstalledPaperlessConnectsSearchesAndPages runs the installed Paperless AI Search extension against a scripted
// Paperless API: without a token it asks for one; saving a valid token goes on to the search prompt; a raw query lists
// the matching documents with a Next Page row and the total.
func TestInstalledPaperlessConnectsSearchesAndPages(t *testing.T) {
	requireUnixProcesses(t)
	dir := filepath.Join("..", "testdata", "extensions", "installed", "local.paperless-ai-search")
	raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	server := fake.NewHTTP()
	server.Handle(paperlessServer())
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	rig.deps.Network = server
	r := servicedRuntime(t, filepath.Join("installed", "local.paperless-ai-search"), rig, installedManifest(t, raw))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	connect := runInstalledView(t, ctx, r, api.Invocation{Ref: api.ActionRef{ID: "search"}})
	if connect.Kind != api.ViewForm || connect.Title != "Connect Paperless" {
		t.Fatalf("without a token the command opened %q (%s), want the Connect Paperless form", connect.Title, connect.Kind)
	}

	response, err := r.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "save-token"}, Form: api.FormValues{"token": {Text: "abc123"}}})
	if err != nil {
		t.Fatalf("save token: %v", err)
	}
	var saved api.Result
	if err := json.Unmarshal(response.Data, &saved); err != nil || saved.Next == nil {
		t.Fatalf("saving a valid token answered %s (%v), want the search prompt as the next step", response.Data, err)
	}

	results := runInstalledView(t, ctx, r, api.Invocation{Ref: api.ActionRef{ID: "search-submit"}, Form: api.FormValues{"query": {Text: "raw: invoice acme"}}})
	if results.Title != "Paperless Search" || results.Subtitle != "Query: invoice acme · Page 1" || results.Source == nil {
		t.Fatalf("results view = %q / %q, want the query shown and a document source", results.Title, results.Subtitle)
	}

	items, err := r.Load(ctx, results.Source.Loader)
	if err != nil {
		t.Fatalf("load results: %v", err)
	}
	var titles []string
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	got := strings.Join(titles, ",")
	if !strings.Contains(got, "Acme Invoice March") || !strings.Contains(got, "Acme Invoice April") || titles[len(titles)-1] != "Next Page" {
		t.Fatalf("results = %v, want both documents then Next Page", titles)
	}
	if next := items[len(items)-1]; next.Subtitle != "45 total matches" {
		t.Fatalf("Next Page subtitle = %q, want the total match count", next.Subtitle)
	}
}
