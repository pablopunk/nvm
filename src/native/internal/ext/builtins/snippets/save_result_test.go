package snippets

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestSaveReturnsToTheListWithoutAFollowUp: a save pops the form, and the list below reloads itself when it is revealed,
// so the save carries no follow-up action of its own.
func TestSaveReturnsToTheListWithoutAFollowUp(t *testing.T) {
	result, err := run(t, New(nil), newContext(), refSave, map[string]string{argID: ""}, saveForm("Signature", ";sig", "Pablo"))
	if err != nil || result.Navigation != api.NavPop || result.Next != nil {
		t.Fatalf("save = %+v, %v; want a pop with no follow-up", result, err)
	}
}
