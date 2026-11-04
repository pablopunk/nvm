package real

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// copyGesture sends Ctrl+C to the focused X11 window with xdotool, after the same focus check the paste gesture makes.
func copyGesture(ctx context.Context, helper platform.Helper, expectedAppID string) error {
	return keyGesture(ctx, helper, expectedAppID, "ctrl+c")
}
