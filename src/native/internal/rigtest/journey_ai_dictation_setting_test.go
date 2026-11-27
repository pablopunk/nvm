//go:build rig

package rigtest

import "testing"

const (
	aiDictationQuestion      = "Change “Clean with AI” in Dictation from off to on"
	aiDictationChanged       = "DICTATION-CLEANUP-ON-MARK"
	aiDictationUnknownEngine = "DICTATION-ENGINE-REFUSED-MARK"
	cleanupSettingTitle      = "Clean with AI"
	dictationSettingsHint    = "Search Dictation Settings"
)

// TestJourneyAIProgramTurnsOnDictationCleanup: Dictation's settings are declared, so Preferences().Set reaches them.
// The approval row asks to turn “Clean with AI” on, Enter allows it, an engine Dictation does not declare is refused
// before anyone is asked, and the Dictation settings page, opened from the Settings page, then shows the switch on.
func TestJourneyAIProgramTurnsOnDictationCleanup(t *testing.T) {
	app, fake := launchSignedInWithScript(t, "", scriptOf(
		programTurn(t, "call_dictation_cleanup", setDictationProgram("cleanupWithAi", "true"), aiDictationChanged),
		programTurn(t, "call_dictation_engine", setDictationProgram("engine", `"whisper"`), aiDictationUnknownEngine),
	))

	askFirstQuestion(t, app, "Clean my dictations with AI")
	waitForApproval(t, app, aiDictationQuestion)
	shoot(t, app, "ai-dictation-1-asks")
	allowApproval(t, app)
	waitForTurnEnd(t, app, aiDictationChanged)
	modelReceived(t, fake, "(the user allowed it)")

	askFollowUp(t, app, "Now transcribe with whisper")
	waitForTurnEnd(t, app, aiDictationUnknownEngine)
	modelReceived(t, fake, "accepts only")

	openRootCommand(t, app, "Settings", "Search Settings")
	must(t, app.Type("Dictation"))
	waitForRow(t, app, "Dictation", elementWait)
	must(t, app.Key("Return"))
	_, err := app.WaitFor(dictationSettingsHint, elementWait)
	must(t, err)
	waitForRow(t, app, cleanupSettingTitle, elementWait)
	shoot(t, app, "ai-dictation-2-settings-page")
	must(t, app.Type("Clean"))
	waitForRow(t, app, cleanupSettingTitle, elementWait)
	_, err = app.WaitFor("On", elementWait)
	must(t, err)
	shoot(t, app, "ai-dictation-3-clean-with-ai-is-on")
}

// setDictationProgram sets one of Dictation's declared settings to the JSON value, by its ids.
func setDictationProgram(setting, value string) string {
	return "_, err := ctx.Preferences().Set(\"nevermind.dictation\", \"" + setting + "\", " + value + ")\nreturn nil, err"
}
