package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"testing"
)

func TestOperationRequirementsDoNotRestoreUnsupportedCommandPrimary(t *testing.T) {
	primary := api.Run("preview", "Preview", nil)
	primary.Requires = []api.Operation{api.OperationFilePreview}
	deps := extview.Deps{OperationsSupported: func(required []api.Operation) bool { return len(required) == 0 }}
	actions := commandMenuActions(deps, "acme.test", extview.MenuSections(&primary, nil, []api.Action{api.Run("remove", "Remove", nil)}))
	if len(actions) != 1 || !actions[0].MenuOnly || actions[0].Title != "Remove" {
		t.Fatalf("unsupported primary restored: %+v", actions)
	}
}
