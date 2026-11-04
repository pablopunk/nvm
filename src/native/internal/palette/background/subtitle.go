package background

import (
	"fmt"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/jobs"
)

// subtitle says what the job is doing, what it runs on and when it last finished, in one line.
func subtitle(snap jobs.Snapshot, at time.Time) string {
	parts := []string{statusText(snap, at), triggerSummary(snap.Triggers)}
	if !snap.LastFinished.IsZero() {
		parts = append(parts, "Last "+ago(at, snap.LastFinished))
	}
	return strings.Join(parts, " · ")
}

func statusText(snap jobs.Snapshot, at time.Time) string {
	switch snap.Status {
	case jobs.StatusRunning:
		return "Running"
	case jobs.StatusBackingOff:
		return "Backing off until " + snap.BackoffUntil.Local().Format("15:04:05")
	case jobs.StatusFailed:
		return "Failed: " + snap.LastError
	case jobs.StatusSucceeded:
		return "Succeeded"
	case jobs.StatusDisabled:
		if snap.Halted != "" {
			return "Halted: " + snap.Halted
		}
		return "Disabled"
	default:
		return "Idle"
	}
}

// triggerSummary names the triggers in words; a job with none runs only when someone runs it.
func triggerSummary(triggers []jobs.Trigger) string {
	if len(triggers) == 0 {
		return "Manual"
	}
	names := make([]string, 0, len(triggers))
	for _, trigger := range triggers {
		names = append(names, triggerName(trigger))
	}
	return strings.Join(names, ", ")
}

func triggerName(trigger jobs.Trigger) string {
	switch trigger.Kind {
	case jobs.TriggerStartup:
		return "Startup"
	case jobs.TriggerInterval:
		return "Every " + duration(trigger.Every)
	case jobs.TriggerEvent:
		return "On " + trigger.Event
	default:
		return string(trigger.Kind)
	}
}

// duration spells a period the way people say it: 2h, 15m, 30s.
func duration(d time.Duration) string {
	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	default:
		return fmt.Sprintf("%ds", d/time.Second)
	}
}

// ago spells how long before now a moment was, in the largest unit that fits.
func ago(now, moment time.Time) string {
	elapsed := now.Sub(moment)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%d min ago", int(elapsed/time.Minute))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(elapsed/time.Hour))
	default:
		return fmt.Sprintf("%d d ago", int(elapsed/(24*time.Hour)))
	}
}
