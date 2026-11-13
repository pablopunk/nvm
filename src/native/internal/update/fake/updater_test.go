package fake

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/update"
)

func TestScriptedChecksAnswerInOrderThenRepeatTheLast(t *testing.T) {
	u := New("2.0.0", update.State{Status: update.StatusAvailable, LatestVersion: "2.1.0"}, update.State{Status: update.StatusIdle})
	if got := u.State(); got.Status != update.StatusIdle {
		t.Fatalf("before any check the state is %q", got.Status)
	}
	if got := u.Check(context.Background()); got.Status != update.StatusAvailable {
		t.Fatalf("first check = %q", got.Status)
	}
	u.Check(context.Background())
	if got := u.Check(context.Background()); got.Status != update.StatusIdle {
		t.Fatalf("third check = %q, want the last scripted state", got.Status)
	}
	if u.Checks() != 3 {
		t.Fatalf("checks = %d, want 3", u.Checks())
	}
}
