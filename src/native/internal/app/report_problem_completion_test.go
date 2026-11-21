package app

import (
	diagnosticsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/diagnostics"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"testing"
)

func TestReportProblemShortcutTargetsDiagnosticsCommand(t *testing.T) {
	ext, cmd, ok := splitExtensionCommandShortcutID(shortcuts.CommandReportProblem)
	if !ok || ext != diagnosticsbuiltin.ID || cmd != diagnosticsbuiltin.ReportAction {
		t.Fatalf("shortcut target = %q %q %v", ext, cmd, ok)
	}
}
