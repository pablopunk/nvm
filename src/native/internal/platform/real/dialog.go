package real

import (
	"strings"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// fileDialog shows MyGo's native open dialog. MyGo blocks until the user dismisses it, so callers run Choose off the
// UI thread.
type fileDialog struct{}

func (fileDialog) Choose(req platform.ChooseRequest) ([]string, error) {
	return mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:             req.Title,
		ButtonLabel:       req.ButtonLabel,
		DefaultPath:       req.DefaultPath,
		Filters:           dialogFilters(req),
		Directory:         req.Folder,
		Multiple:          req.Multiple,
		CreateDirectories: req.CreateFolders,
	})
}

// dialogFilters returns the one filter that limits the files shown, or none when the request names no extensions.
func dialogFilters(req platform.ChooseRequest) []mygo.FileFilter {
	if len(req.Extensions) == 0 {
		return nil
	}
	extensions := make([]string, 0, len(req.Extensions))
	for _, extension := range req.Extensions {
		extensions = append(extensions, strings.TrimPrefix(extension, "."))
	}
	return []mygo.FileFilter{{Name: req.FilterName, Extensions: extensions}}
}
