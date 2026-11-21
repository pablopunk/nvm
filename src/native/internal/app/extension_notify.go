package app

import (
	"os"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/palette/feedback"
	"github.com/pablopunk/nvm/native/internal/platform/folders"
)

// legacyAppFolderName is the folder 1.x (the Electron app, named nvm) kept its user data in.
const legacyAppFolderName = "nvm"

// notifyExtensionFailure shows a failure of an extension's action in the HUD, under the extension's title. The extension's
// goroutine calls it, so the HUD is drawn on the UI thread.
func (p *App) notifyExtensionFailure(title, message string) {
	p.redraw(func() { p.showFeedback(title, message, feedback.ToneError) })
}

// legacyStatePath is the 1.x state file, which records whether each 1.x extension file was on.
func (p *App) legacyStatePath() string {
	return filepath.Join(folders.ElectronUserData(p.goos, userHomeDir(), os.Getenv, legacyAppFolderName), "state.json")
}
