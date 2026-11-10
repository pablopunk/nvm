package child

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestRefreshProducesValidHostAction(t *testing.T) {
	action := (remoteViews{}).Refresh()
	if action.Kind != api.ActionRefreshView {
		t.Fatalf("kind = %q", action.Kind)
	}
	if err := action.Validate(); err != nil {
		t.Fatalf("refresh action: %v", err)
	}
}
