//go:build rig

package rigtest

import "testing"

const (
	aiJobTitle        = "AI Conversation Cleanup"
	aiJobQuestion     = "Turn off the background job “" + aiJobTitle + "”"
	aiJobsListed      = "JOBS-LISTED-MARK"
	aiJobRefused      = "JOB-REFUSED-MARK"
	aiJobTurnedOff    = "JOB-OFF-MARK"
	listJobTitles     = "var titles []string\nfor _, j := range ctx.Jobs().List() {\n\ttitles = append(titles, j.Title)\n}\nreturn titles, nil"
	turnOffCleanupJob = `for _, j := range ctx.Jobs().List() {
	if j.Title == "` + aiJobTitle + `" {
		return j.ID, ctx.Jobs().SetEnabled(j.ID, false)
	}
}
return "no such job", nil`
)

// TestJourneyAIProgramListsJobsAndTurnsOneOff (#326): the AI lists the background jobs without asking, then turns the AI
// conversation cleanup off. SetEnabled is a Setting call, so it asks every time: Ctrl+D refuses the first, Enter allows
// the second, and the Background Tasks page shows the job disabled.
func TestJourneyAIProgramListsJobsAndTurnsOneOff(t *testing.T) {
	app, fake := launchSignedInWithScript(t, "", scriptOf(
		programTurn(t, "call_jobs_list", listJobTitles, aiJobsListed),
		programTurn(t, "call_job_refused", turnOffCleanupJob, aiJobRefused),
		programTurn(t, "call_job_off", turnOffCleanupJob, aiJobTurnedOff),
	))

	askFirstQuestion(t, app, "Which background jobs do I have")
	waitForTurnEnd(t, app, aiJobsListed)
	modelReceived(t, fake, aiJobTitle)
	shoot(t, app, "ai-jobs-1-listed-without-asking")

	askFollowUp(t, app, "Turn off the AI conversation cleanup")
	waitForApproval(t, app, aiJobQuestion)
	shoot(t, app, "ai-jobs-2-asks")
	refuseApproval(t, app)
	waitForTurnEnd(t, app, aiJobRefused)
	modelReceived(t, fake, "the user declined")

	askFollowUp(t, app, "Try again, I will allow it")
	waitForApproval(t, app, aiJobQuestion)
	allowApproval(t, app)
	waitForTurnEnd(t, app, aiJobTurnedOff)
	modelReceived(t, fake, "turned off the background job")

	openRootCommand(t, app, "Background Tasks", "Search background tasks")
	must(t, app.Type(aiJobTitle))
	waitForRow(t, app, aiJobTitle, elementWait)
	_, err := app.WaitFor("disabled · ", elementWait)
	must(t, err)
	shoot(t, app, "ai-jobs-3-background-tasks-shows-it-disabled")
}
