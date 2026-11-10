package builder

import (
	"strings"
	"testing"
)

func TestMigrationTableNamesRenamedAndDroppedMembers(t *testing.T) {
	for _, want := range []string{"ctx.actions.openPath -> func:OpenPath", "ctx.actions.native -> gone"} {
		if !strings.Contains(migration1x, want) {
			t.Errorf("the table lacks %q", want)
		}
	}
}
