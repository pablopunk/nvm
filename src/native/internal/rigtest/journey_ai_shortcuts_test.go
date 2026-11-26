//go:build rig

package rigtest

import (
	"testing"
	"time"
)

// findClipboardHistory finds the Clipboard History command among the bindable ones, as the guidance tells the model.
const findClipboardHistory = `id := ""
for _, c := range ctx.Shortcuts().Bindable() {
	if c.Title == "Clipboard History" {
		id = c.ActionID
	}
}
if id == "" {
	return "no such command", nil
}
`

const (
	aiShortcut         = "Control+Alt+Shift+H"
	aiShortcutQuestion = "for “Clipboard History”"
	aiShortcutRefused  = "SHORTCUT-REFUSED-MARK"
	aiShortcutAllowed  = "SHORTCUT-ALLOWED-MARK"
	setClipboardHotkey = findClipboardHistory + `bound, err := ctx.Shortcuts().Set(id, "` + aiShortcut + `")
if err != nil {
	return nil, err
}
return bound.Label, nil`
)

// TestJourneyAIProgramSetsAGlobalShortcut (#324): the AI binds Clipboard History to a global shortcut. The approval row
// names the command; Ctrl+D refuses the first try and the model hears the decline, Enter allows the second, and the
// new shortcut, pressed on the desktop, opens Clipboard History with the copied text.
func TestJourneyAIProgramSetsAGlobalShortcut(t *testing.T) {
	app, fake := launchSignedInWithScript(t, "", scriptOf(
		programTurn(t, "call_set_refused", setClipboardHotkey, aiShortcutRefused),
		programTurn(t, "call_set_allowed", setClipboardHotkey, aiShortcutAllowed),
	))
	clipboardText := uniqueClipboardText("ai shortcut")
	must(t, app.ClipboardSet(clipboardText))

	askFirstQuestion(t, app, "Bind clipboard history to a shortcut")
	waitForApproval(t, app, aiShortcutQuestion)
	shoot(t, app, "ai-shortcut-1-asks")
	refuseApproval(t, app)
	waitForTurnEnd(t, app, aiShortcutRefused)
	modelReceived(t, fake, "the user declined")
	shoot(t, app, "ai-shortcut-2-refused")

	askFollowUp(t, app, "Try again, I will allow it")
	waitForApproval(t, app, aiShortcutQuestion)
	allowApproval(t, app)
	waitForTurnEnd(t, app, aiShortcutAllowed)
	modelReceived(t, fake, "the user allowed it")
	shoot(t, app, "ai-shortcut-3-allowed")

	hidePalette(t, app)
	must(t, app.PressHotkey(aiShortcut))
	waitForRow(t, app, clipboardText, clipWait)
	shoot(t, app, "ai-shortcut-4-hotkey-opens-clipboard-history")
}

const (
	aiAlias              = "zzq"
	aiAliasAddQuestion   = "Add alias “" + aiAlias + "” for “Clipboard History”"
	aiAliasDelQuestion   = "Remove alias “" + aiAlias + "” from “Clipboard History”"
	aiAliasRefused       = "ALIAS-REFUSED-MARK"
	aiAliasAdded         = "ALIAS-ADDED-MARK"
	aiAliasTaken         = "ALIAS-TAKEN-MARK"
	aiAliasRemoved       = "ALIAS-REMOVED-MARK"
	addClipboardAlias    = findClipboardHistory + `return nil, ctx.Shortcuts().AddAlias(id, "` + aiAlias + `")`
	removeClipboardAlias = `return nil, ctx.Shortcuts().RemoveAlias("` + aiAlias + `")`
	aliasSearchSettle    = time.Second
)

// TestJourneyAIProgramAddsAndRemovesAnAlias (#326): the AI adds the alias zzq to Clipboard History. Ctrl+D refuses the
// first try, Enter allows the second, and a third try is refused before anyone is asked because the alias is taken.
// Typing zzq at the root then finds Clipboard History. A second chat removes the alias, and zzq finds it no more.
func TestJourneyAIProgramAddsAndRemovesAnAlias(t *testing.T) {
	app, fake := launchSignedInWithScript(t, "", scriptOf(
		programTurn(t, "call_alias_refused", addClipboardAlias, aiAliasRefused),
		programTurn(t, "call_alias_added", addClipboardAlias, aiAliasAdded),
		programTurn(t, "call_alias_taken", addClipboardAlias, aiAliasTaken),
		programTurn(t, "call_alias_removed", removeClipboardAlias, aiAliasRemoved),
	))

	askFirstQuestion(t, app, "Give clipboard history the alias zzq")
	waitForApproval(t, app, aiAliasAddQuestion)
	shoot(t, app, "ai-alias-1-asks")
	refuseApproval(t, app)
	waitForTurnEnd(t, app, aiAliasRefused)
	modelReceived(t, fake, "the user declined")

	askFollowUp(t, app, "Try again, I will allow it")
	waitForApproval(t, app, aiAliasAddQuestion)
	allowApproval(t, app)
	waitForTurnEnd(t, app, aiAliasAdded)
	modelReceived(t, fake, "the user allowed it")

	askFollowUp(t, app, "Add it once more")
	waitForTurnEnd(t, app, aiAliasTaken)
	modelReceived(t, fake, "is already an alias of Clipboard History")
	shoot(t, app, "ai-alias-2-taken-refused-without-asking")

	searchRootFor(t, app, aiAlias)
	waitForRow(t, app, "Clipboard History", elementWait)
	shoot(t, app, "ai-alias-3-root-finds-clipboard-history")

	hidePalette(t, app)
	askFirstQuestion(t, app, "Remove the alias zzq")
	waitForApproval(t, app, aiAliasDelQuestion)
	allowApproval(t, app)
	waitForTurnEnd(t, app, aiAliasRemoved)
	modelReceived(t, fake, "removed alias")

	searchRootFor(t, app, aiAlias)
	time.Sleep(aliasSearchSettle)
	assertNoRow(t, app, "Clipboard History")
	shoot(t, app, "ai-alias-4-root-no-longer-finds-it")
}

// searchRootFor types query at a freshly summoned root and waits for the root's own Automate row, so the search ran.
func searchRootFor(t *testing.T, app *App, query string) {
	t.Helper()
	hidePalette(t, app)
	must(t, app.SummonPalette())
	must(t, app.Type(query))
	waitSearchValue(t, app, query)
	waitForRow(t, app, `Automate "`+query+`"`, elementWait)
}
