package real

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// UNTESTED on a Mac: the helper operations are the ones 1.x ships (selected-text.swift); the Go side is covered with
// a scripted helper. expectedAppID is not checked on macOS: the helper acts on the app in front.
func copyGesture(ctx context.Context, helper platform.Helper, _ string) error {
	return macCopySelection(ctx, helper, macosHelperPath())
}

func accessibleSelection(ctx context.Context, helper platform.Helper) (string, error) {
	return macAccessibleSelection(ctx, helper, macosHelperPath())
}
