package extitems

import (
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// cadenceOf returns how often an extension's items render on their own, and the issues found in its triggers. The
// shortest interval wins. Startup and RefreshMenuBar render at load and on request, so they need no cadence. Other
// trigger types are disabled here until the trigger host (WP3.6) serves them.
func cadenceOf(triggers []api.Trigger) (time.Duration, []string) {
	var every time.Duration
	var issues []string
	for _, trigger := range triggers {
		if err := trigger.Validate(); err != nil {
			issues = append(issues, err.Error())
			continue
		}
		switch trigger.Type {
		case api.TriggerInterval:
			every = shorterOf(every, intervalOf(trigger))
		case api.TriggerStartup:
		default:
			issues = append(issues, fmt.Sprintf("trigger %q is not available in menu bars yet", trigger.Type))
		}
	}
	return every, issues
}

// intervalOf parses an interval trigger that Validate already accepted.
func intervalOf(trigger api.Trigger) time.Duration {
	duration, _ := time.ParseDuration(trigger.Every)
	return duration
}

// shorterOf returns the smaller positive duration; zero means "not set yet".
func shorterOf(current, candidate time.Duration) time.Duration {
	if current == 0 || candidate < current {
		return candidate
	}
	return current
}
