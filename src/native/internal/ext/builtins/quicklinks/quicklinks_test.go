package quicklinks

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeStorage is an in-memory extension storage holding JSON values by key.
type fakeStorage struct {
	api.Storage
	values map[string][]byte
	failed bool
}

func (s *fakeStorage) Get(key string, into any) (bool, error) {
	raw, ok := s.values[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, into)
}

func (s *fakeStorage) Set(key string, value any) error {
	if s.failed {
		return errors.New("disk full")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.values[key] = raw
	return nil
}

// fakeNavigation records the view a handler pushes or the pop it asks for.
type fakeNavigation struct {
	api.Navigation
}

func (fakeNavigation) Push(view *api.View) api.Result {
	return api.Result{View: view, Navigation: api.NavPush}
}

func (fakeNavigation) Pop() api.Result { return api.Result{Navigation: api.NavPop} }

// fakeContext serves storage and navigation; every other service is absent.
type fakeContext struct {
	api.Context
	storage *fakeStorage
}

func (c fakeContext) Storage() api.Storage       { return c.storage }
func (c fakeContext) Navigation() api.Navigation { return fakeNavigation{} }

// saved decodes the links the storage holds.
func saved(t *testing.T, ctx api.Context) []Link {
	t.Helper()
	var links []Link
	if _, err := ctx.Storage().Get(storageKey, &links); err != nil {
		t.Fatal(err)
	}
	return links
}

func newContext() fakeContext {
	return fakeContext{storage: &fakeStorage{values: map[string][]byte{}}}
}

func searchLinks(t *testing.T, ctx api.Context, query string) []api.Item {
	t.Helper()
	items, err := New(nil).SearchItems(ctx, query)
	if err != nil {
		t.Fatalf("SearchItems(%q): %v", query, err)
	}
	return items
}

func TestKeywordWithTermOpensTheDefaultSite(t *testing.T) {
	items := searchLinks(t, newContext(), "gh nevermind tests")
	if len(items) != 1 {
		t.Fatalf("items = %d, want the GitHub row", len(items))
	}
	item := items[0]
	if item.Title != "Search GitHub for “nevermind tests”" || item.Subtitle != "Quicklink" || item.Icon != "link" || item.Score != 980 {
		t.Fatalf("row = %+v", item)
	}
	if item.Primary == nil || item.Primary.Kind != api.ActionOpenURL || item.Primary.URL != "https://github.com/search?q=nevermind+tests" {
		t.Fatalf("primary = %+v, want the escaped GitHub search", item.Primary)
	}
}

func TestKeywordMatchIgnoresCase(t *testing.T) {
	if items := searchLinks(t, newContext(), "GH nevermind"); len(items) != 1 {
		t.Fatalf("items = %d, want one row for an upper-case keyword", len(items))
	}
}

func TestKeywordWithoutTermMatchesNothing(t *testing.T) {
	for _, query := range []string{"", "gh", "gh   "} {
		if items := searchLinks(t, newContext(), query); len(items) != 0 {
			t.Fatalf("query %q: items = %+v, want none", query, items)
		}
	}
}

func TestUserLinksComeBeforeDefaultsAndKeepTheirOwnTemplate(t *testing.T) {
	legacy := []Link{{Name: "Hacker News", Keyword: "hn", URL: "https://hn.algolia.com/?q={query}"}}
	items, err := New(legacy).SearchItems(newContext(), "hn rust")
	if err != nil || len(items) != 1 || items[0].Title != "Search Hacker News for “rust”" {
		t.Fatalf("items = %+v, err %v; want the legacy link", items, err)
	}
	if got := items[0].Primary.URL; got != "https://hn.algolia.com/?q=rust" {
		t.Fatalf("url = %q", got)
	}
}

func TestLegacyLinksAnswerUntilTheFirstSave(t *testing.T) {
	legacy := []Link{{Name: "Hacker News", Keyword: "hn", URL: "https://hn.algolia.com/?q={query}"}}
	ctx := newContext()
	if items, _ := New(legacy).SearchItems(ctx, "hn rust"); len(items) != 1 {
		t.Fatalf("legacy link not found: %+v", items)
	}
	if _, stored := ctx.storage.values[storageKey]; stored {
		t.Fatal("reading the legacy links wrote them to storage")
	}
}

func TestCreateCommandIsSearchOnlyBlueAndNamedAsBefore(t *testing.T) {
	commands, err := Extension{}.Commands(newContext())
	if err != nil || len(commands) != 1 {
		t.Fatalf("commands = %+v, err %v", commands, err)
	}
	command := commands[0]
	if command.Title != "Create Quicklink" || command.Appearance == nil || command.Appearance.Foreground != api.ColorBlue {
		t.Fatalf("command = %+v", command)
	}
	if len(command.Placement) != 1 || command.Placement[0] != api.PlacementSearch {
		t.Fatalf("placement = %v, want search only", command.Placement)
	}
}

func TestCreateCommandOpensTheFormWithItsThreeFields(t *testing.T) {
	result, err := Extension{}.RunAction(newContext(), api.Invocation{Ref: api.ActionRef{ID: CreateCommandID}})
	if err != nil || result.View == nil || result.Navigation != api.NavPush {
		t.Fatalf("result = %+v, err %v", result, err)
	}
	if result.View.Kind != api.ViewForm || len(result.View.Form.Fields) != 3 || result.View.Form.Submit == nil {
		t.Fatalf("view = %+v, want a form with three fields", result.View)
	}
	for _, field := range result.View.Form.Fields {
		if !field.Required {
			t.Errorf("field %q is not required", field.ID)
		}
	}
}

func saveForm(t *testing.T, ctx api.Context, name, keyword, address string) (api.Result, error) {
	t.Helper()
	return Extension{}.RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: saveActionID}, Form: api.FormValues{
		fieldName: api.TextValue(name), fieldKeyword: api.TextValue(keyword), fieldURL: api.TextValue(address),
	}})
}

func TestSavedLinkIsSearchableAndKeepsTheLegacyLinks(t *testing.T) {
	legacy := []Link{{Name: "Hacker News", Keyword: "hn", URL: "https://hn.algolia.com/?q={query}"}}
	ctx := newContext()
	result, err := Extension{legacy: legacy}.RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: saveActionID}, Form: api.FormValues{
		fieldName: api.TextValue("  Docs "), fieldKeyword: api.TextValue("d"), fieldURL: api.TextValue("https://docs.example/?q={query}"),
	}})
	if err != nil || result.Navigation != api.NavPop {
		t.Fatalf("save = %+v, err %v; want a pop", result, err)
	}
	if items := searchLinks(t, ctx, "d rust"); len(items) != 1 || items[0].Title != "Search Docs for “rust”" {
		t.Fatalf("items = %+v, want the saved link", items)
	}
	if len(saved(t, ctx)) != 2 {
		t.Fatalf("stored = %+v, want the legacy link and the new one", saved(t, ctx))
	}
}

func TestSaveRefusesAFormMissingPart(t *testing.T) {
	for _, form := range [][3]string{{"", "d", "https://a.example"}, {"Docs", " ", "https://a.example"}, {"Docs", "d", "ftp://a.example"}} {
		if _, err := saveForm(t, newContext(), form[0], form[1], form[2]); !errors.Is(err, api.ErrInvalidInput) {
			t.Errorf("form %q: err = %v, want invalid input", form, err)
		}
	}
}

func TestStorageFailureIsReturnedNotHidden(t *testing.T) {
	ctx := newContext()
	ctx.storage.failed = true
	if _, err := saveForm(t, ctx, "Docs", "d", "https://a.example"); err == nil {
		t.Fatal("save succeeded although storage refused the write")
	}
}

func TestUnknownActionIsRefused(t *testing.T) {
	if _, err := (Extension{}).RunAction(newContext(), api.Invocation{Ref: api.ActionRef{ID: "nope"}}); !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("err = %v, want invalid input", err)
	}
}

func TestSearchURLEscapesTheTermAsAQueryValue(t *testing.T) {
	got := searchURL(Link{URL: "https://x.example/?q={query}"}, "a&b c")
	if got != "https://x.example/?q="+url.QueryEscape("a&b c") {
		t.Fatalf("url = %q", got)
	}
}
