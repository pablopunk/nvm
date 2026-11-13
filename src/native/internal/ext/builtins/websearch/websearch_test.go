package websearch

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// fakeSettings serves one engine value; a nil value is a setting that cannot be read.
type fakeSettings struct {
	api.Settings
	value *string
}

func (s fakeSettings) Get(id string, into any) error {
	if s.value == nil {
		return errors.New("settings unavailable")
	}
	*into.(*string) = *s.value
	return nil
}

// fakeContext serves the settings; every other service is absent.
type fakeContext struct {
	api.Context
	settings fakeSettings
}

func (c fakeContext) Settings() api.Settings { return c.settings }

func contextWithEngine(key string) fakeContext {
	return fakeContext{settings: fakeSettings{value: &key}}
}

func searchRowsFor(t *testing.T, ctx api.Context, query string) []api.Item {
	t.Helper()
	items, err := New("").SearchItems(ctx, query)
	if err != nil {
		t.Fatalf("SearchItems(%q): %v", query, err)
	}
	return items
}

func TestSearchRowIsAQueryFallbackOfTheDefaultEngine(t *testing.T) {
	items := searchRowsFor(t, contextWithEngine("google"), "nevermind tests")
	if len(items) != 1 {
		t.Fatalf("items = %d, want the search row", len(items))
	}
	item := items[0]
	if item.Title != `Search the web for "nevermind tests"` || item.Subtitle != "Search instead" || item.Icon != "search" {
		t.Fatalf("row = %+v", item)
	}
	if !item.QueryFallback || item.Section != search.KindSearch || item.LearnKey != SearchLearnKey {
		t.Fatalf("row = %+v, want a query fallback in the search section, learned as web:search", item)
	}
	if item.Primary == nil || item.Primary.Kind != api.ActionOpenURL || item.Primary.Title != "Search the Web" ||
		item.Primary.URL != "https://www.google.com/search?q=nevermind+tests" {
		t.Fatalf("primary = %+v", item.Primary)
	}
}

func TestSearchUsesTheChosenEngine(t *testing.T) {
	items := searchRowsFor(t, contextWithEngine("bing"), "a&b")
	if got := items[0].Primary.URL; got != "https://www.bing.com/search?q=a%26b" {
		t.Fatalf("url = %q, want Bing with the query escaped", got)
	}
}

func TestUnreadableSettingFallsBackToGoogle(t *testing.T) {
	items := searchRowsFor(t, fakeContext{}, "rust")
	if got := items[0].Primary.URL; !strings.HasPrefix(got, "https://www.google.com/search?q=") {
		t.Fatalf("url = %q, want Google", got)
	}
}

func TestAddressOpensTheWebsiteInsteadOfSearching(t *testing.T) {
	items := searchRowsFor(t, contextWithEngine("google"), "github.com/pablopunk")
	if len(items) != 1 {
		t.Fatalf("items = %+v, want the website row only", items)
	}
	item := items[0]
	if item.Title != "Open github.com/pablopunk" || item.Subtitle != "Open website" || item.Icon != "globe" || item.Section != search.KindWeb {
		t.Fatalf("row = %+v", item)
	}
	if item.QueryFallback || item.Score != scoreWebURL || item.Appearance == nil || item.Appearance.Foreground != api.ColorBlue {
		t.Fatalf("row = %+v, want a blue website row at 150, not a fallback", item)
	}
	if item.Primary == nil || item.Primary.Title != "Open Website" {
		t.Fatalf("primary = %+v", item.Primary)
	}
}

func TestEmptyQueryHasNoRows(t *testing.T) {
	if items := searchRowsFor(t, contextWithEngine("google"), ""); len(items) != 0 {
		t.Fatalf("items = %+v, want none", items)
	}
}

func TestLongQueryTitleIsCutToOneLine(t *testing.T) {
	query := strings.Repeat("word ", 30)
	title := searchItem(query, engineNamed(DefaultEngine)).Title
	if !strings.HasSuffix(title, "…\"") || len([]rune(title)) > len(`Search the web for ""`)+maxTitleRunes+1 {
		t.Fatalf("title = %q, want one cut line", title)
	}
}

func TestEngineCyclesInTheSettingsOrder(t *testing.T) {
	want := map[string]string{"google": "duckduckgo", "duckduckgo": "bing", "bing": "brave", "brave": "google", "nope": "duckduckgo"}
	for from, to := range want {
		if got := NextEngine(from); got != to {
			t.Errorf("NextEngine(%q) = %q, want %q", from, got, to)
		}
	}
}

func TestEngineNameFallsBackToGoogle(t *testing.T) {
	if EngineName("bing") != "Bing" || EngineName("") != "Google" || EngineName("nope") != "Google" {
		t.Fatal("engine names do not match the settings row")
	}
}

func TestManifestDefaultsToTheLegacyEngine(t *testing.T) {
	definition := New("brave").Manifest().Settings[0]
	var value string
	if err := json.Unmarshal(definition.Value, &value); err != nil || value != "brave" {
		t.Fatalf("default = %q (%v), want the legacy brave", value, err)
	}
	if definition.ID != EngineSettingID || definition.Type != api.SettingString {
		t.Fatalf("setting = %+v", definition)
	}
	if got := New("").Manifest().Settings[0].Value; string(got) != `"google"` {
		t.Fatalf("default without a legacy engine = %s", got)
	}
}
