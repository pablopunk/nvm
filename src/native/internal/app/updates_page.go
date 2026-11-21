package app

import (
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	updatesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/updates"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/update"
)

// updatesPage is the Updates page, pushed from Settings, the root and the Check for Updates command. The Updates
// built-in declares its rows, and the palette draws them.
func (p *App) updatesPage() *palette.View {
	fallback := &palette.View{Title: "Updates", Placeholder: "Search updates", Search: func(string) []search.Item { return nil }}
	return p.builtInOpenPage(updatesbuiltin.ID, fallback)
}

// errNoUpdateToDownload refuses a download when none is under way or finished. The updater starts a download as soon as
// a check finds a newer version, so a download is never waiting to be started.
var errNoUpdateToDownload = errors.New("no update is waiting to download")

// runUpdateOp performs the update operation an Updates built-in action names, and returns the reason it refused.
func (p *App) runUpdateOp(op string) error {
	switch op {
	case api.UpdateOpCheck:
		p.checkForUpdates(func(string) {})
		return nil
	case api.UpdateOpDownload:
		return p.downloadUpdate()
	case api.UpdateOpInstall:
		return p.restartToUpdate()
	case api.UpdateOpChannel:
		return p.toggleUpdateChannel()
	}
	return fmt.Errorf("there is no update operation %q", op)
}

// downloadUpdate accepts a download request while an update is under way or downloaded, and refuses it otherwise.
func (p *App) downloadUpdate() error {
	switch p.updateSnapshot().State.Status {
	case update.StatusDownloading, update.StatusDownloaded:
		return nil
	}
	return errNoUpdateToDownload
}
