package app

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

// Accessory words for the states an extension can be in. Each one is shown in the list and in the detail pane.
const (
	stateRunning          = "Running"
	stateStarting         = "Starting"
	stateRestarting       = "Restarting"
	stateBroken           = "Broken"
	stateDisabledFailures = "Disabled after failures"
	stateDisabled         = "Disabled"
	stateInvalid          = "Invalid"
)

// extensionStateLabel says what an extension is doing in one word; it is the row's accessory.
func extensionStateLabel(l manager.Listing) string {
	switch {
	case len(l.Problems) > 0:
		return stateInvalid
	case !l.Enabled:
		return stateDisabled
	case l.UnsupportedHere:
		// Not an error: the extension is on and works on the OS it is for. Here it just does not run.
		return l.OnlyOn
	}
	switch l.Runtime.State {
	case supervisor.StateRunning, supervisor.StateDormant:
		return stateRunning
	case supervisor.StateBroken:
		return stateBroken
	case supervisor.StateDisabled:
		return stateDisabledFailures
	case supervisor.StateFailed:
		return stateRestarting
	}
	return stateStarting
}

// extensionReason is why an extension is not running, or "" when nothing is wrong. Problems come first: an
// invalid folder never starts, so its runtime has nothing to say.
func extensionReason(l manager.Listing) string {
	if len(l.Problems) > 0 {
		return strings.Join(l.Problems, "; ")
	}
	if !l.Enabled || l.UnsupportedHere {
		return ""
	}
	switch l.Runtime.State {
	case supervisor.StateBroken, supervisor.StateDisabled, supervisor.StateFailed:
		return l.Runtime.Reason
	}
	return ""
}

// extensionSubtitle is the row's second line: the id, and the reason after it when the extension is not running.
func extensionSubtitle(l manager.Listing) string {
	if reason := extensionReason(l); reason != "" {
		return l.ID + " · " + reason
	}
	return l.ID
}

func extensionTitle(l manager.Listing) string {
	if l.Title != "" {
		return l.Title
	}
	return l.ID
}

// extensionCanEnable is true for an extension that is off and loads, so Enable can start it.
func extensionCanEnable(l manager.Listing) bool {
	return !l.Enabled && len(l.Problems) == 0
}

// extensionDisabledByFailures is true when the runtime gave up after repeated failures. Reset clears it.
func extensionDisabledByFailures(l manager.Listing) bool {
	return l.Enabled && len(l.Problems) == 0 && l.Runtime.State == supervisor.StateDisabled
}

// extensionTone colors the row's tile by how the extension is doing.
func extensionTone(l manager.Listing) string {
	switch extensionStateLabel(l) {
	case stateRunning:
		return "green"
	case stateRestarting:
		return "orange"
	case stateBroken, stateDisabledFailures, stateInvalid:
		return "red"
	}
	return "gray"
}
