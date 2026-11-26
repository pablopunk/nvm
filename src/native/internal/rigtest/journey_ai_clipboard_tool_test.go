//go:build rig

package rigtest

import (
	"strings"
	"testing"
)

const (
	aiClipboardRefused = "CLIPBOARD-REFUSED-MARK"
	aiClipboardRead    = "CLIPBOARD-READ-MARK"
	// aiClipboardQuery is what the program searches for. The copied text adds a unique suffix the program never names,
	// so only a read that ran can bring the whole text to the model.
	aiClipboardQuery = "rig sensitive"
)

// searchClipboardTwice searches the clipboard history for query twice in one program: a SensitiveRead tool asks once
// per run and extension, so only the first call may ask.
func searchClipboardTwice(query string) string {
	return `args := map[string]any{"query": "` + query + `"}
first, err := ctx.Tools().Call("nevermind.clipboard", "search-clipboard", args)
if err != nil {
	return nil, err
}
second, err := ctx.Tools().Call("nevermind.clipboard", "search-clipboard", args)
return []any{first, second}, err`
}

// TestJourneyAIProgramReadsClipboardHistoryAfterOneQuestion (#330): clipboard search is a SensitiveRead tool. Ctrl+D
// refuses the first program's read and the model hears the decline. The second program reads twice, Enter answers the
// one question it asks, the turn ends without another question, and the copied text reaches the model.
func TestJourneyAIProgramReadsClipboardHistoryAfterOneQuestion(t *testing.T) {
	copied := uniqueClipboardText(aiClipboardQuery)
	question := "Read the clipboard history items that contain “" + aiClipboardQuery + "”"
	app, fake := launchSignedInWithScript(t, "", scriptOf(
		programTurn(t, "call_clipboard_refused", searchClipboardTwice(aiClipboardQuery), aiClipboardRefused),
		programTurn(t, "call_clipboard_read", searchClipboardTwice(aiClipboardQuery), aiClipboardRead),
	))
	must(t, app.ClipboardSet(copied))
	openRootCommand(t, app, "Clipboard History", "Search Clipboard History")
	waitForRow(t, app, copied, clipWait)
	hidePalette(t, app)

	askFirstQuestion(t, app, "What did I copy about the rig")
	waitForApproval(t, app, question)
	shoot(t, app, "ai-clipboard-1-asks")
	refuseApproval(t, app)
	waitForTurnEnd(t, app, aiClipboardRefused)
	modelReceived(t, fake, "the user declined")
	if strings.Contains(lastProgramResult(t, fake), copied) {
		t.Fatal("the refused read sent the copied text to the model")
	}

	askFollowUp(t, app, "Try again, I will allow it")
	waitForApproval(t, app, question)
	allowApproval(t, app)
	waitForTurnEnd(t, app, aiClipboardRead)
	modelReceived(t, fake, copied)
	shoot(t, app, "ai-clipboard-2-read-after-one-question")
}
