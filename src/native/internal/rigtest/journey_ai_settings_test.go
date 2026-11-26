//go:build rig

package rigtest

import "testing"

const (
	aiSettingQuestion    = "Change “Clipboard History Size” in Settings from “300 items” to “500 items”"
	aiSettingRefused     = "SETTING-REFUSED-MARK"
	aiSettingChanged     = "SETTING-CHANGED-MARK"
	aiSettingOutOfRange  = "SETTING-OUT-OF-RANGE-MARK"
	clipboardSizeSetting = "Clipboard History Size"
)

// TestJourneyAIProgramChangesADeclaredSetting (#327, #329): Preferences().Set changes Clipboard History Size from 300 to
// 500. The approval row names both values in the words of the setting's choices. Ctrl+D refuses the first try, Enter
// allows the second, and a size the setting does not declare is refused before anyone is asked. The Settings page then
// shows 500 items.
func TestJourneyAIProgramChangesADeclaredSetting(t *testing.T) {
	app, fake := launchSignedInWithScript(t, "", scriptOf(
		programTurn(t, "call_setting_refused", setClipboardSizeProgram("500"), aiSettingRefused),
		programTurn(t, "call_setting_changed", setClipboardSizeProgram("500"), aiSettingChanged),
		programTurn(t, "call_setting_out_of_range", setClipboardSizeProgram("12345"), aiSettingOutOfRange),
	))

	askFirstQuestion(t, app, "Keep 500 clipboard items")
	waitForApproval(t, app, aiSettingQuestion)
	shoot(t, app, "ai-setting-1-asks")
	refuseApproval(t, app)
	waitForTurnEnd(t, app, aiSettingRefused)
	modelReceived(t, fake, "the user declined")

	askFollowUp(t, app, "Try again, I will allow it")
	waitForApproval(t, app, aiSettingQuestion)
	allowApproval(t, app)
	waitForTurnEnd(t, app, aiSettingChanged)
	modelReceived(t, fake, "(the user allowed it)")

	askFollowUp(t, app, "Now keep 12345 items")
	waitForTurnEnd(t, app, aiSettingOutOfRange)
	modelReceived(t, fake, "accepts only")
	shoot(t, app, "ai-setting-2-undeclared-value-refused-without-asking")

	openRootCommand(t, app, "Settings", "Search Settings")
	must(t, app.Type(clipboardSizeSetting))
	waitForRow(t, app, clipboardSizeSetting, elementWait)
	_, err := app.WaitFor("500 items", elementWait)
	must(t, err)
	shoot(t, app, "ai-setting-3-settings-page-shows-500-items")
}

// setClipboardSizeProgram sets the Settings built-in's clipboard history size to size, by its ids.
func setClipboardSizeProgram(size string) string {
	return `_, err := ctx.Preferences().Set("nevermind.settings", "clipboardLimit", "` + size + `")` + "\nreturn nil, err"
}
