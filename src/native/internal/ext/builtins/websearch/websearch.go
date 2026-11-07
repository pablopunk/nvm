// Package websearch is the built-in that offers the web fallbacks of the palette: the website a typed address names,
// and a search of the default engine for any other query. The default engine is a declared setting, so the settings
// view cycles it and the host reads it through the same service a folder extension uses.
//
// Layer: ext. The package imports api, search and the standard library.
package websearch

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity.
const ID = "nevermind.web"

// EngineSettingID is the declared setting that holds the key of the default engine.
const EngineSettingID = "engine"

// DefaultEngine is the key of the engine a query searches when the user has chosen none.
const DefaultEngine = "google"

// SearchLearnKey is the usage key of the search row, which the learner ranks by.
const SearchLearnKey = "web:search"

// scoreWebURL is the band of the website row, above the commands for an address the user typed.
const scoreWebURL = 150

// Engine is one search engine: its key in the setting, its name and its search address, which has {query} for the
// search text.
type Engine struct {
	Key, Name, URL string
}

// engines are the engines the setting cycles, in the order the settings row offers them.
var engines = []Engine{
	{Key: "google", Name: "Google", URL: "https://www.google.com/search?q={query}"},
	{Key: "duckduckgo", Name: "DuckDuckGo", URL: "https://duckduckgo.com/?q={query}"},
	{Key: "bing", Name: "Bing", URL: "https://www.bing.com/search?q={query}"},
	{Key: "brave", Name: "Brave Search", URL: "https://search.brave.com/search?q={query}"},
}

// Engines returns the engines in the order the settings row offers them, as key and name pairs.
func Engines() []Engine {
	return append([]Engine(nil), engines...)
}

// NextEngine is the key of the engine after key in the order the settings row cycles them. An unknown key starts at the
// first engine.
func NextEngine(key string) string {
	i := indexOfEngine(key)
	return engines[(i+1)%len(engines)].Key
}

// EngineName is the name of the engine with key, or of the default engine when none has it.
func EngineName(key string) string {
	return engineNamed(key).Name
}

// engineNamed is the engine with key, or the first engine when none has it.
func engineNamed(key string) Engine {
	return engines[indexOfEngine(key)]
}

// indexOfEngine is the position of key among the engines, or 0 for a key none has.
func indexOfEngine(key string) int {
	for i, engine := range engines {
		if engine.Key == key {
			return i
		}
	}
	return 0
}

// Extension is the web built-in. legacyEngine is the engine key the settings file held before this built-in; it is the
// setting's default, so a user who never chose an engine keeps the one they had.
type Extension struct {
	legacyEngine string
}

// New returns the built-in whose default engine is the one the settings file named.
func New(legacyEngine string) Extension {
	return Extension{legacyEngine: legacyEngine}
}

// Manifest identifies the built-in and declares its engine setting. The host writes the setting from the settings view,
// which needs the settings.write capability.
func (e Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion: "1.0", Identity: api.Identity{ID: ID, Title: "Web"},
		Capabilities: []api.Capability{api.CapabilitySettingsWrite},
		Settings: []api.SettingDefinition{{
			ID: EngineSettingID, Title: "Default Search Engine", Type: api.SettingString, Value: engineValue(e.legacyEngine),
		}},
	}
}

// engineValue is the JSON default of the engine setting: the legacy engine when it names one, else the default engine.
func engineValue(legacyEngine string) json.RawMessage {
	key := DefaultEngine
	if legacyEngine != "" {
		key = engineNamed(legacyEngine).Key
	}
	raw, _ := json.Marshal(key)
	return raw
}

// SearchItems answers a typed query with the website it names, or with the search of the default engine. The empty
// query has none.
func (Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if query == "" {
		return nil, nil
	}
	if address := search.URLFromQuery(query); address != "" {
		return []api.Item{websiteItem(address)}, nil
	}
	return []api.Item{searchItem(query, currentEngine(ctx))}, nil
}

// currentEngine is the engine the setting names. A setting that cannot be read falls back to the default.
func currentEngine(ctx api.Context) Engine {
	var key string
	if err := ctx.Settings().Get(EngineSettingID, &key); err != nil {
		return engineNamed(DefaultEngine)
	}
	return engineNamed(key)
}

// websiteItem is the row that opens an address the query names, titled with the address without its scheme.
func websiteItem(address string) api.Item {
	open := api.OpenURL("Open Website", address)
	return api.Item{
		Title: "Open " + strings.TrimPrefix(strings.TrimPrefix(address, "https://"), "http://"), Subtitle: "Open website", Icon: "globe",
		Section: search.KindWeb, Score: scoreWebURL, Appearance: &api.Appearance{Foreground: api.ColorBlue},
		Primary: &open,
	}
}

// searchItem is the fallback row that searches the engine for the whole query. It is a query fallback, so the host
// places it below the rows the query matches.
func searchItem(query string, engine Engine) api.Item {
	open := api.OpenURL("Search the Web", searchURL(engine, query))
	return api.Item{
		Title: `Search the web for "` + oneLine(query) + `"`, Subtitle: "Search instead", Icon: "search",
		Section: search.KindSearch, LearnKey: SearchLearnKey, QueryFallback: true,
		Appearance: &api.Appearance{Foreground: api.ColorBlue},
		Primary:    &open,
	}
}

// searchURL is the engine's address for query, with the query escaped as a query value.
func searchURL(engine Engine, query string) string {
	return strings.ReplaceAll(engine.URL, "{query}", url.QueryEscape(query))
}
