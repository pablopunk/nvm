package quicklinks

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func linkToolCall(name, args string) api.AIToolCall {
	return api.AIToolCall{Name: name, Args: json.RawMessage(args)}
}

func TestQuicklinkToolsDeclareTheirClasses(t *testing.T) {
	manifest := New(nil).Manifest()
	if err := manifest.ValidateBuiltIn(); err != nil {
		t.Fatal(err)
	}
	want := map[string]api.AIToolClass{toolList: api.AIToolRead, toolCreate: api.AIToolWrite, toolUpdate: api.AIToolWrite, toolDelete: api.AIToolDestructive}
	for _, tool := range manifest.AITools {
		if manifest.EnforcedAIToolClass(tool) != want[tool.Name] {
			t.Errorf("%s runs as %s; want %s", tool.Name, manifest.EnforcedAIToolClass(tool), want[tool.Name])
		}
	}
}

func TestQuicklinkToolsCreateUpdateAndDelete(t *testing.T) {
	ext, ctx := New(nil), newContext()
	if _, err := ext.CallAITool(ctx, linkToolCall(toolCreate, `{"name":"Hacker News","keyword":"hn","url":"https://hn.algolia.com/?q={query}"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := ext.CallAITool(ctx, linkToolCall(toolUpdate, `{"keyword":"hn","newKeyword":"news"}`)); err != nil {
		t.Fatal(err)
	}
	if links := saved(t, ctx); len(links) != 1 || links[0].Keyword != "news" || links[0].Name != "Hacker News" {
		t.Fatalf("links = %+v", links)
	}
	listed, _ := ext.CallAITool(ctx, linkToolCall(toolList, `{}`))
	if all, _ := listed.([]toolLink); len(all) != 1+len(defaultLinks) || all[0].BuiltIn || !all[1].BuiltIn {
		t.Fatalf("listed = %+v", listed)
	}
	if _, err := ext.CallAITool(ctx, linkToolCall(toolDelete, `{"keyword":"news"}`)); err != nil || len(saved(t, ctx)) != 0 {
		t.Fatalf("err = %v, links %+v", err, saved(t, ctx))
	}
}

// A keyword another link has, a built-in link and a link with no address are refused while the call is described.
func TestQuicklinkToolsRefuseConflictsBeforeAsking(t *testing.T) {
	ext, ctx := New([]Link{{Name: "Docs", Keyword: "docs", URL: "https://docs.example/?q={query}"}}), newContext()
	cases := map[string]struct {
		call   api.AIToolCall
		reason string
	}{
		"keyword of a default": {linkToolCall(toolCreate, `{"name":"Mine","keyword":"GH","url":"https://x.example/{query}"}`), "already used by the quicklink “GitHub”"},
		"keyword of the user":  {linkToolCall(toolCreate, `{"name":"Mine","keyword":"docs","url":"https://x.example/{query}"}`), "already used by the quicklink “Docs”"},
		"rename onto another":  {linkToolCall(toolUpdate, `{"keyword":"docs","newKeyword":"yt"}`), "already used by the quicklink “YouTube”"},
		"delete a default":     {linkToolCall(toolDelete, `{"keyword":"wiki"}`), "built-in quicklink"},
		"no address":           {linkToolCall(toolCreate, `{"name":"Mine","keyword":"mine","url":"ftp://x"}`), "http(s) URL"},
		"unknown":              {linkToolCall(toolDelete, `{"keyword":"nope"}`), "list them first"},
	}
	for name, c := range cases {
		if _, err := ext.DescribeAITool(ctx, c.call); !errors.Is(err, api.ErrInvalidInput) || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: err = %v; want %q", name, err, c.reason)
		}
	}
	described, err := ext.DescribeAITool(ctx, linkToolCall(toolDelete, `{"keyword":"docs"}`))
	if err != nil || described != "Delete the quicklink “Docs” (keyword “docs”) for https://docs.example/?q={query}" {
		t.Fatalf("described = %q, %v", described, err)
	}
}
