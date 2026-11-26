package snippets

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func snippetToolCall(name, args string) api.AIToolCall {
	return api.AIToolCall{Name: name, Args: json.RawMessage(args)}
}

func callSnippetTool(t *testing.T, ext Extension, ctx api.Context, name, args string) (any, error) {
	t.Helper()
	return ext.CallAITool(ctx, snippetToolCall(name, args))
}

func TestSnippetToolsDeclareTheirClasses(t *testing.T) {
	manifest := New(nil).Manifest()
	if err := manifest.ValidateBuiltIn(); err != nil {
		t.Fatal(err)
	}
	want := map[string]api.AIToolClass{toolSearch: api.AIToolRead, toolCreate: api.AIToolWrite, toolUpdate: api.AIToolWrite, toolDelete: api.AIToolDestructive}
	for _, tool := range manifest.AITools {
		if manifest.EnforcedAIToolClass(tool) != want[tool.Name] {
			t.Errorf("%s runs as %s; want %s", tool.Name, manifest.EnforcedAIToolClass(tool), want[tool.Name])
		}
	}
}

func TestSnippetToolsCreateSearchUpdateAndDelete(t *testing.T) {
	ext, ctx := New(nil), newContext()
	created, err := callSnippetTool(t, ext, ctx, toolCreate, `{"name":"Signature","keyword":"SIG","text":"Best, Ana"}`)
	sn, _ := created.(toolSnippet)
	if err != nil || sn.Keyword != "sig" || sn.ID == "" {
		t.Fatalf("created = %+v, %v", created, err)
	}
	found, _ := callSnippetTool(t, ext, ctx, toolSearch, `{"query":"best ana"}`)
	if matches, _ := found.([]toolSnippet); len(matches) != 1 || matches[0].ID != sn.ID {
		t.Fatalf("found = %+v", found)
	}
	if _, err := callSnippetTool(t, ext, ctx, toolUpdate, `{"id":"`+sn.ID+`","text":"Thanks, Ana"}`); err != nil {
		t.Fatal(err)
	}
	if got := stored(t, ctx); len(got) != 1 || got[0].Text != "Thanks, Ana" || got[0].Name != "Signature" || got[0].Keyword != "sig" {
		t.Fatalf("stored = %+v; an update must keep the fields it leaves out", got)
	}
	if _, err := callSnippetTool(t, ext, ctx, toolDelete, `{"id":"`+sn.ID+`"}`); err != nil || len(stored(t, ctx)) != 0 {
		t.Fatalf("err = %v, stored %+v", err, stored(t, ctx))
	}
}

// A keyword another snippet holds is refused while the call is described, so the user is never asked about a change
// that would fail, and the reason names the snippet that holds it.
func TestSnippetToolsRefuseAKeywordAnotherSnippetHas(t *testing.T) {
	ext, ctx := New(nil), newContext()
	first, _ := callSnippetTool(t, ext, ctx, toolCreate, `{"name":"Signature","keyword":"sig","text":"Best"}`)
	second, _ := callSnippetTool(t, ext, ctx, toolCreate, `{"name":"Address","keyword":"addr","text":"Main St"}`)
	for name, call := range map[string]api.AIToolCall{
		"create": snippetToolCall(toolCreate, `{"name":"Other","keyword":"Sig","text":"x"}`),
		"update": snippetToolCall(toolUpdate, `{"id":"`+second.(toolSnippet).ID+`","keyword":"sig"}`),
	} {
		_, err := ext.DescribeAITool(ctx, call)
		if !errors.Is(err, api.ErrInvalidInput) || !strings.Contains(err.Error(), "already used by “Signature”") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := ext.DescribeAITool(ctx, snippetToolCall(toolUpdate, `{"id":"`+first.(toolSnippet).ID+`","keyword":"sig","name":"Sign-off"}`)); err != nil {
		t.Fatalf("a snippet keeping its own keyword was refused: %v", err)
	}
}

func TestSnippetToolDescriptionsNameTheSnippet(t *testing.T) {
	ext, ctx := New(nil), newContext()
	created, _ := callSnippetTool(t, ext, ctx, toolCreate, `{"name":"Signature","keyword":"sig","text":"Best"}`)
	id := created.(toolSnippet).ID
	cases := map[string]struct{ call, want string }{
		toolDelete: {`{"id":"` + id + `"}`, "Delete the snippet “Signature” (keyword “sig”)"},
		toolUpdate: {`{"id":"` + id + `","name":"Sign-off","keyword":""}`, "Change the snippet “Signature”: name to “Sign-off”, keyword removed"},
		toolCreate: {`{"name":"Address","text":"Main St"}`, "Create the snippet “Address”"},
	}
	for name, c := range cases {
		described, err := ext.DescribeAITool(ctx, snippetToolCall(name, c.call))
		if err != nil || described != c.want {
			t.Errorf("%s: described = %q, %v; want %q", name, described, err, c.want)
		}
	}
	if _, err := ext.DescribeAITool(ctx, snippetToolCall(toolDelete, `{"id":"snip-gone"}`)); !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("a missing snippet: err = %v", err)
	}
}
