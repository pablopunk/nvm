package services

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// NotifyFunc shows a failure of an extension's action to the user: the extension's title and the reason. The call
// comes from the extension's goroutine and must not block.
type NotifyFunc func(title, message string)

// automationDeniedMarkers are the osascript messages for an Automation grant that macOS refuses or never got.
var automationDeniedMarkers = []string{"-1743", "Not authorized to send Apple events"}

// automationHint is what the user does when macOS refuses the grant that AppleScript needs.
const automationHint = "Nevermind needs permission to control System Events. Allow it in System Settings > Privacy & Security > Automation."

// maxReasonRunes bounds the failure line the log and the HUD show from the program's own error text.
const maxReasonRunes = 200

// reportAppleScriptFailure makes a failed AppleScript visible: a warning in the log with the reason, and the same reason
// in the HUD under the extension's title. Extensions often ignore the exit code, so the host cannot rely on them to show it.
func (h *Handler) reportAppleScriptFailure(result api.ShellResult) {
	reason := appleScriptReason(result)
	h.logger.Warn("AppleScript failed", logx.F{"exit": result.ExitCode, "reason": reason})
	if h.deps.Notify != nil {
		h.deps.Notify(h.manifest.Title, reason)
	}
}

// appleScriptReason explains a failed AppleScript for the user: the Automation hint when macOS refused the grant,
// otherwise the first line the program printed, otherwise the exit code.
func appleScriptReason(result api.ShellResult) string {
	if containsAnyMarker(result.Stderr, automationDeniedMarkers) {
		return automationHint
	}
	if line := firstNonEmptyLine(result.Stderr); line != "" {
		return "AppleScript failed: " + truncateReason(line)
	}
	return fmt.Sprintf("AppleScript failed with exit code %d", result.ExitCode)
}

func containsAnyMarker(text string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func firstNonEmptyLine(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func truncateReason(line string) string {
	runes := []rune(line)
	if len(runes) <= maxReasonRunes {
		return line
	}
	return string(runes[:maxReasonRunes]) + "..."
}
