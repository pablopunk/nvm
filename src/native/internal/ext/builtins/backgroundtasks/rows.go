package backgroundtasks

import (
	"fmt"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// clockLayout is how the page shows a time of day, in the local zone.
const clockLayout = "15:04:05"

// jobRows is one row per job, in the order the host lists them.
func jobRows(ctx api.Context) []api.Item {
	jobs := ctx.Jobs().List()
	rows := make([]api.Item, 0, len(jobs))
	for _, job := range jobs {
		rows = append(rows, jobRow(job))
	}
	return rows
}

// jobRow is one job: its status, its triggers and its last run as the subtitle. Enter shows its details and its runs.
func jobRow(job api.Job) api.Item {
	details := api.PushView("Show Details", api.PreviewView(job.Title, jobDetailsMarkdown(job)))
	actions := []api.Action{details, runJobAction(job), toggleJobAction(job)}
	if job.LastError != "" {
		actions = append(actions, clearErrorAction(job))
	}
	return api.Item{
		ID: "job:" + job.ID, Title: job.Title, Subtitle: jobSubtitle(job), Icon: jobIcon(job),
		Accessories: []api.Accessory{{Text: ownerLabel(job.Owner)}},
		Primary:     api.Primary(details), Actions: actions,
	}
}

func runJobAction(job api.Job) api.Action {
	return api.Run(runAction, "Run Now", map[string]string{argID: job.ID})
}

// toggleJobAction turns a job off when it runs and on when it does not. A halted job is off, so it reads as on.
func toggleJobAction(job api.Job) api.Action {
	if job.Enabled && job.Halted == "" {
		return api.Run(toggleAction, "Disable Job", map[string]string{argID: job.ID, argEnabled: "false"})
	}
	return api.Run(toggleAction, "Enable Job", map[string]string{argID: job.ID, argEnabled: "true"})
}

func clearErrorAction(job api.Job) api.Action {
	return api.Run(clearAction, "Clear Error", map[string]string{argID: job.ID})
}

// jobSubtitle says what the job is doing, what it runs on and when it last finished, in one line.
func jobSubtitle(job api.Job) string {
	return statusText(job) + " · " + triggerSummary(job.Triggers) + " · Last: " + timeOrNever(job.LastFinishedAt)
}

// statusText says the status in the words the page uses. A failure names its error, and a halted job names its reason.
func statusText(job api.Job) string {
	switch {
	case job.Status == api.JobStatusRunning:
		return "Running"
	case job.Status == api.JobStatusFailed:
		return "Failed: " + orDefault(job.LastError, "unknown error")
	case job.Status == api.JobStatusBackingOff:
		return "Backing off until " + timeOrNever(job.BackoffUntil)
	case job.Halted != "":
		return "Halted: " + job.Halted
	}
	return job.Status
}

// triggerSummary names the triggers in words; a job with none runs only when someone runs it.
func triggerSummary(triggers []api.JobTrigger) string {
	if len(triggers) == 0 {
		return "Manual"
	}
	names := make([]string, 0, len(triggers))
	for _, trigger := range triggers {
		names = append(names, triggerName(trigger))
	}
	return strings.Join(names, " · ")
}

func triggerName(trigger api.JobTrigger) string {
	switch trigger.Kind {
	case api.JobTriggerStartup:
		return "Startup"
	case api.JobTriggerInterval:
		return fmt.Sprintf("Every %ds", (trigger.EveryMs+500)/1000)
	case api.JobTriggerEvent:
		return "On " + trigger.Event
	}
	return "Manual"
}

// jobIcon is the icon of a job's row for its status, from the icons the palette draws.
func jobIcon(job api.Job) string {
	switch job.Status {
	case api.JobStatusRunning:
		return "activity"
	case api.JobStatusFailed:
		return "circle-alert"
	case api.JobStatusBackingOff:
		return "clock"
	case api.JobStatusDisabled:
		return "power"
	case api.JobStatusSucceeded:
		return "check"
	}
	return "timer"
}

// ownerLabel names who runs a job: the host, or an extension.
func ownerLabel(owner string) string {
	if owner == api.JobOwnerExtension {
		return "Extension"
	}
	return "Host"
}

// timeOrNever shows a time of day, or Never for a job that has not reached it yet.
func timeOrNever(at time.Time) string {
	if at.IsZero() {
		return "Never"
	}
	return at.Local().Format(clockLayout)
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
