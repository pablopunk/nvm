package system

import "testing"

// TestEveryHostOpMapsToItsCommand keeps the ops extensions name in step with the commands the host runs.
func TestEveryHostOpMapsToItsCommand(t *testing.T) {
	for _, op := range []string{"lock-screen", "sleep", "restart", "shut-down", "log-out", "empty-trash", "open-system-settings"} {
		if id, ok := CommandForOp(op); !ok || string(id) != op {
			t.Errorf("CommandForOp(%q) = %q, %v", op, id, ok)
		}
	}
}

// TestUnknownOpsHaveNoCommand refuses an op the host does not run, so it is never guessed at.
func TestUnknownOpsHaveNoCommand(t *testing.T) {
	for _, op := range []string{"", "quit", "made-up"} {
		if _, ok := CommandForOp(op); ok {
			t.Errorf("CommandForOp(%q) found a command", op)
		}
	}
}

// TestSettingsCommandTakesTheOSLabel names the settings command with the label the composition root passes in.
func TestSettingsCommandTakesTheOSLabel(t *testing.T) {
	if got := Title(CommandSystemPanes, "Open Settings"); got != "Open Settings" {
		t.Fatalf("Title = %q, want the OS label", got)
	}
	if got := Title(CommandEmptyTrash, ""); got != "Empty Trash" {
		t.Fatalf("Title(empty-trash) = %q", got)
	}
}
