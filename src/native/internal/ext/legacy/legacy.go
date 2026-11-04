// Package legacy reads the extension state 1.x kept, so a folder converted from a 1.x source file keeps the state that
// file had. It only reads: nothing here writes to the 1.x data.
//
// Layer: ext. The package imports platform and the standard library only.
package legacy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// stateDocument is the part of 1.x's state.json that records the extension files: the file name of each one and
// whether it was on.
type stateDocument struct {
	ExtensionManager struct {
		Files map[string]struct {
			Enabled bool `json:"enabled"`
		} `json:"files"`
	} `json:"extensionManager"`
}

// EnabledFiles returns whether each 1.x extension file was on, keyed by its file name. found is false when the 1.x
// state file does not exist, which means 1.x was never run on this machine.
func EnabledFiles(files platform.FS, statePath string) (enabled map[string]bool, found bool, err error) {
	raw, err := files.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read the 1.x state: %w", err)
	}
	var document stateDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, true, fmt.Errorf("the 1.x state is not valid JSON: %w", err)
	}
	enabled = make(map[string]bool, len(document.ExtensionManager.Files))
	for name, entry := range document.ExtensionManager.Files {
		enabled[name] = entry.Enabled
	}
	return enabled, true, nil
}
