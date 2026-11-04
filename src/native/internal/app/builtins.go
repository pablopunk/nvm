package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/syspanes"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/systemcommands"
)

// builtInExtensions are the extensions compiled into the app. They run in the host, are always on, and answer the
// root and the palette through the same manager as folder extensions.
func builtInExtensions() []api.Extension {
	return []api.Extension{syspanes.Extension{}, systemcommands.Extension{}}
}
