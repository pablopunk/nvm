package real

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// selectedText is the production SelectedText port. The copy gesture is the OS's own; the file for each OS says how
// it is sent.
type selectedText struct {
	helper platform.Helper
}

// NewSelectedText returns the production selected-text port, which sends the OS copy gesture through helper.
func NewSelectedText(helper platform.Helper) platform.SelectedText {
	return selectedText{helper: helper}
}

// CopySelection sends the copy gesture of this OS to the frontmost app.
func (s selectedText) CopySelection(expectedAppID string) error {
	return copyGesture(context.Background(), s.helper, expectedAppID)
}

// ReadAccessibleSelection reads the selection of the frontmost app through the OS accessibility API.
func (s selectedText) ReadAccessibleSelection() (string, error) {
	return accessibleSelection(context.Background(), s.helper)
}
