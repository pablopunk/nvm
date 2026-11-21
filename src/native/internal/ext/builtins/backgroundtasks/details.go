package backgroundtasks

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// jobDetailsMarkdown is the page a job's Show Details opens: its identity, its state and its last run, then its recent runs.
func jobDetailsMarkdown(job api.Job) string {
	lines := []string{"# " + job.Title, "", "- ID: " + job.ID, "- Owner: " + ownerLabel(job.Owner)}
	if job.Scope != "" {
		lines = append(lines, "- Scope: "+job.Scope)
	}
	lines = append(lines,
		"- Status: "+job.Status,
		"- Enabled: "+yesNo(job.Enabled),
		"- Running: "+yesNo(job.Running),
		"- Triggers: "+triggerSummary(job.Triggers),
		fmt.Sprintf("- Runs: %d", job.Runs),
		fmt.Sprintf("- Failures: %d", job.Failures),
		"- Last reason: "+orDefault(job.LastReason, "-"),
		"- Last started: "+timeOrNever(job.LastStartedAt),
		"- Last finished: "+timeOrNever(job.LastFinishedAt),
	)
	if !job.LastFinishedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("- Last duration: %dms", job.LastDurationMs))
	}
	if !job.NextRunAt.IsZero() {
		lines = append(lines, "- Next run: "+timeOrNever(job.NextRunAt))
	}
	if !job.BackoffUntil.IsZero() {
		lines = append(lines, "- Backoff until: "+timeOrNever(job.BackoffUntil))
	}
	lines = append(lines, fmt.Sprintf("- Consecutive failures: %d", job.ConsecutiveFailures))
	if job.LastError != "" {
		lines = append(lines, "", "## Last error", "", job.LastError)
	}
	lines = append(lines, "", "## Recent runs", "", runHistoryMarkdown(job.History))
	return strings.Join(lines, "\n")
}

// runHistoryMarkdown lists the runs, or says none is recorded yet.
func runHistoryMarkdown(history []api.JobRun) string {
	if len(history) == 0 {
		return "_No runs recorded yet._"
	}
	rows := make([]string, 0, len(history))
	for _, run := range history {
		rows = append(rows, runMarkdown(run))
	}
	return strings.Join(rows, "\n")
}

func runMarkdown(run api.JobRun) string {
	row := fmt.Sprintf("- %s · %s · %s · %dms", timeOrNever(run.FinishedAt), run.Outcome, run.Reason, run.DurationMs)
	if run.Error != "" {
		row += " · " + run.Error
	}
	return row
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
