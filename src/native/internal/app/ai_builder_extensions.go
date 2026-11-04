package app

import (
	"github.com/pablopunk/nvm/native/internal/ai/builder"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
)

// builderExtensions is the extension manager as the builder uses it. Until the manager has started, every call
// reports builder.ErrExtensionsLoading. The manager locks itself, so the builder may call it off the UI thread.
type builderExtensions struct {
	app *App
}

var _ builder.Extensions = builderExtensions{}

func (e builderExtensions) manager() (*manager.Manager, error) {
	if m := e.app.extensionManager(); m != nil {
		return m, nil
	}
	return nil, builder.ErrExtensionsLoading
}

func (e builderExtensions) Definition(id string) (api.Manifest, string, error) {
	m, err := e.manager()
	if err != nil {
		return api.Manifest{}, "", err
	}
	return m.Definition(id)
}

func (e builderExtensions) Check(id string, files map[string][]byte) ([]string, error) {
	m, err := e.manager()
	if err != nil {
		return nil, err
	}
	return m.Check(id, files)
}

func (e builderExtensions) Install(id string, files map[string][]byte, enable bool) error {
	m, err := e.manager()
	if err != nil {
		return err
	}
	if err := m.Install(id, files, enable); err != nil {
		return err
	}
	e.app.refreshPreview(id)
	return nil
}

// Catalog lists the installed extensions for the builder's prompt. Until the manager has started there are none to list.
func (e builderExtensions) Catalog() []builder.Installed {
	m := e.app.extensionManager()
	if m == nil {
		return nil
	}
	entries := m.Catalog()
	installed := make([]builder.Installed, 0, len(entries))
	for _, entry := range entries {
		installed = append(installed, builder.Installed{ID: entry.ID, Title: entry.Title, Enabled: entry.Enabled, Titles: entry.Titles})
	}
	return installed
}
