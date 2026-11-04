package background

import (
	"fmt"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/search"
)

// listRow is one job: its status and triggers, and the actions that act on it. Enter, the first action, opens its
// history.
func listRow(source Source, snap jobs.Snapshot, now func() time.Time) search.Item {
	return search.Item{
		ID:        "job:" + snap.ID,
		Title:     snap.Title,
		Subtitle:  subtitle(snap, now()),
		Icon:      iconFor(snap.Status),
		Accessory: ownerLabel(snap.Owner),
		Actions:   rowActions(source, snap, now),
	}
}

func rowActions(source Source, snap jobs.Snapshot, now func() time.Time) []search.Action {
	id, title := snap.ID, snap.Title
	actions := []search.Action{
		{Title: "Show History", Run: func(h search.Host) { h.Push(Details(source, id)) }},
		{Title: "Run Now", Dismiss: search.DismissNever, Run: func(h search.Host) {
			if err := source.RunNow(id); err != nil {
				h.Toast(err.Error())
				return
			}
			h.Toast("Ran " + title)
			h.Refresh()
		}},
	}
	if snap.Enabled {
		actions = append(actions, search.Action{Title: "Disable Job", Dismiss: search.DismissNever, Run: func(h search.Host) {
			toggle(h, source, id, title, false)
		}})
	} else {
		actions = append(actions, search.Action{Title: "Enable Job", Dismiss: search.DismissNever, Run: func(h search.Host) {
			toggle(h, source, id, title, true)
		}})
	}
	if snap.LastError != "" {
		actions = append(actions, search.Action{Title: "Clear Error", Dismiss: search.DismissNever, Run: func(h search.Host) {
			if err := source.ClearError(id); err != nil {
				h.Toast(err.Error())
				return
			}
			h.Toast("Cleared " + title)
			h.Refresh()
		}})
	}
	return actions
}

func toggle(h search.Host, source Source, id, title string, enabled bool) {
	if err := source.SetEnabled(id, enabled); err != nil {
		h.Toast(err.Error())
		return
	}
	if enabled {
		h.Toast("Enabled " + title)
	} else {
		h.Toast("Disabled " + title)
	}
	h.Refresh()
}

// historyRow is one run: its outcome and reason, then when it finished, how long it took and its error.
func historyRow(entry jobs.HistoryEntry) search.Item {
	return search.Item{
		ID:        fmt.Sprintf("run:%d", entry.StartedAt.UnixNano()),
		Title:     outcomeLabel(entry.Outcome) + " · " + entry.Reason,
		Subtitle:  historySubtitle(entry),
		Icon:      outcomeIcon(entry.Outcome),
		Accessory: entry.Duration.Round(time.Millisecond).String(),
	}
}

func historySubtitle(entry jobs.HistoryEntry) string {
	parts := []string{entry.FinishedAt.Local().Format("2006-01-02 15:04:05")}
	if entry.Error != "" {
		parts = append(parts, entry.Error)
	}
	return strings.Join(parts, " · ")
}

func outcomeLabel(outcome jobs.Outcome) string {
	switch outcome {
	case jobs.OutcomeFailed:
		return "Failed"
	case jobs.OutcomeSkipped:
		return "Skipped"
	default:
		return "Succeeded"
	}
}

func outcomeIcon(outcome jobs.Outcome) string {
	switch outcome {
	case jobs.OutcomeFailed:
		return "circle-alert"
	case jobs.OutcomeSkipped:
		return "clock"
	default:
		return "check"
	}
}

func iconFor(status jobs.Status) string {
	switch status {
	case jobs.StatusRunning:
		return "activity"
	case jobs.StatusFailed:
		return "circle-alert"
	case jobs.StatusBackingOff:
		return "clock"
	case jobs.StatusDisabled:
		return "power"
	case jobs.StatusSucceeded:
		return "check"
	default:
		return "timer"
	}
}

func ownerLabel(owner jobs.Owner) string {
	if owner == jobs.OwnerExtension {
		return "Extension"
	}
	return "Host"
}
