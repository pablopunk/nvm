package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/builtins/appcommands"
	placesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/places"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/systemcommands"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// systemRowActionIDs names the system command rows by their stable action ID, keyed by the row's ID in the system
// commands built-in.
var systemRowActionIDs = map[string]string{
	"lock-screen": shortcuts.CommandLockScreen,
	"sleep":       shortcuts.CommandSleep,
	"restart":     shortcuts.CommandRestart,

	"open-system-settings": shortcuts.CommandSystemSettings,
}

// placeRowActionIDs names the folder rows by their stable action ID, keyed by the folder's name.
var placeRowActionIDs = map[string]string{
	"Downloads": shortcuts.CommandPlaceDownloads,
	"Documents": shortcuts.CommandPlaceDocuments,
	"Desktop":   shortcuts.CommandPlaceDesktop,
}

// builtInRowActionID is the stable action ID of a row a command-placed built-in lists, or "" when the row has none.
// Shortcuts and aliases bind to this ID, never to the row's title.
func builtInRowActionID(builtIn string, row search.Item) string {
	switch builtIn {
	case systemcommands.ID:
		return systemRowActionIDs[row.ID]
	case placesbuiltin.ID:
		return placeRowActionIDs[row.Title]
	case appcommands.ID:
		if row.ID == appcommands.QuitOp {
			return shortcuts.CommandQuitNevermind
		}
		if row.ID == appcommands.RestartOp {
			return shortcuts.CommandRestartNevermind
		}
	}
	return ""
}
