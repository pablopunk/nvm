package child

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestLearningsFixtureShowsRowsWithEditAndConfirmedDelete(t *testing.T) {
	result := runFixtureCommand(t, "acme.learnings", "list")
	if result.View == nil || result.View.Kind != api.ViewList || len(result.View.Items) != 3 {
		t.Fatalf("view = %+v; want a list of three rules", result.View)
	}
	row := result.View.Items[1]
	if row.Primary == nil || row.Primary.Ref.ID != "edit" || len(row.Actions) != 1 || row.Actions[0].Confirm == nil || !row.Actions[0].Confirm.Destructive {
		t.Fatalf("row = %+v; want Edit and a confirmed Delete", row)
	}
	if err := result.View.Validate(); err != nil {
		t.Fatalf("the list does not validate: %v", err)
	}
}

func TestLearningsFixtureEmptyStateIsOnTheEmptyList(t *testing.T) {
	result := runFixtureCommand(t, "acme.learnings", "empty")
	if result.View == nil || len(result.View.Items) != 0 || result.View.EmptyView == nil {
		t.Fatalf("view = %+v; want an empty list with an empty state", result.View)
	}
}
