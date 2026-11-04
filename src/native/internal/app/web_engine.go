package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/builtins/websearch"
)

// webEngineKey is the key of the default search engine, as the web built-in's setting holds it. Without the built-in
// it is the default engine.
func (p *App) webEngineKey() string {
	m := p.extensionManager()
	if m == nil {
		return websearch.DefaultEngine
	}
	var key string
	if err := m.ReadSetting(websearch.ID, websearch.EngineSettingID, &key); err != nil {
		return websearch.DefaultEngine
	}
	return key
}

// cycleWebEngine stores the engine after the current default in the built-in's setting.
func (p *App) cycleWebEngine() error {
	m := p.extensionManager()
	if m == nil {
		return nil
	}
	return m.WriteSetting(websearch.ID, websearch.EngineSettingID, websearch.NextEngine(p.webEngineKey()))
}
