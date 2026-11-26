//go:build rig

package rigtest

import "testing"

const (
	aiSnippetName     = "Rig greeting"
	aiSnippetKeyword  = "rgreet"
	aiSnippetQuestion = "Delete the snippet “" + aiSnippetName + "” (keyword “" + aiSnippetKeyword + "”)"
	aiQuicklinkName   = "Rig Docs"
	aiQuicklinkKey    = "rdocs"
	aiToolsListed     = "TOOLS-LISTED-MARK"
	aiSnippetCreated  = "SNIPPET-CREATED-MARK"
	aiSnippetConflict = "SNIPPET-CONFLICT-MARK"
	aiQuicklinkMade   = "QUICKLINK-CREATED-MARK"
	aiSnippetKept     = "SNIPPET-KEPT-MARK"
	aiSnippetDeleted  = "SNIPPET-DELETED-MARK"

	listEveryTool = `var names []string
for _, tool := range ctx.Tools().List("") {
	names = append(names, tool.ExtensionID+"/"+tool.Name+"/"+string(tool.Class))
}
return names, nil`
	createGreeting = `return ctx.Tools().Call("nevermind.snippets", "create-snippet", map[string]any{"name": "` + aiSnippetName +
		`", "keyword": "` + aiSnippetKeyword + `", "text": "Hello from the AI"})`
	createGreetingConflict = `return ctx.Tools().Call("nevermind.snippets", "create-snippet", map[string]any{"name": "Rig twin", "keyword": "` +
		aiSnippetKeyword + `", "text": "Same keyword"})`
	createDocsQuicklink = `return ctx.Tools().Call("nevermind.quicklinks", "create-quicklink", map[string]any{"name": "` + aiQuicklinkName +
		`", "keyword": "` + aiQuicklinkKey + `", "url": "https://example.com/search?q={query}"})`
	deleteGreeting = `found, err := ctx.Tools().Call("nevermind.snippets", "search-snippets", map[string]any{"query": "` + aiSnippetName + `"})
if err != nil {
	return nil, err
}
list, _ := found.([]any)
if len(list) == 0 {
	return "no such snippet", nil
}
first, _ := list[0].(map[string]any)
return ctx.Tools().Call("nevermind.snippets", "delete-snippet", map[string]any{"id": first["id"]})`
)

// TestJourneyAIProgramUsesSnippetsAndQuicklinksTools (#330): the AI lists the extensions' tools, creates a snippet and a
// quicklink without asking (Write), and a second snippet with the same keyword is refused before anyone is asked. The
// Snippets command shows the one snippet, and typing the quicklink's keyword at the root offers it. A second chat
// deletes the snippet, a Destructive tool that asks every time: Ctrl+D keeps it, Enter deletes it.
func TestJourneyAIProgramUsesSnippetsAndQuicklinksTools(t *testing.T) {
	app, fake := launchSignedInWithScript(t, "", scriptOf(
		programTurn(t, "call_tools_list", listEveryTool, aiToolsListed),
		programTurn(t, "call_snippet_create", createGreeting, aiSnippetCreated),
		programTurn(t, "call_snippet_conflict", createGreetingConflict, aiSnippetConflict),
		programTurn(t, "call_quicklink_create", createDocsQuicklink, aiQuicklinkMade),
		programTurn(t, "call_snippet_delete_refused", deleteGreeting, aiSnippetKept),
		programTurn(t, "call_snippet_delete_allowed", deleteGreeting, aiSnippetDeleted),
	))

	askFirstQuestion(t, app, "What can you do with my extensions")
	waitForTurnEnd(t, app, aiToolsListed)
	modelReceived(t, fake, "nevermind.snippets/create-snippet/write", "nevermind.snippets/delete-snippet/destructive",
		"nevermind.quicklinks/create-quicklink/write", "nevermind.clipboard/search-clipboard/sensitive-read")

	askFollowUp(t, app, "Save a greeting snippet")
	waitForTurnEnd(t, app, aiSnippetCreated)
	modelReceived(t, fake, aiSnippetName)

	askFollowUp(t, app, "Save another one with the same keyword")
	waitForTurnEnd(t, app, aiSnippetConflict)
	modelReceived(t, fake, "is already used by")
	shoot(t, app, "ai-tools-1-keyword-conflict-refused-without-asking")

	askFollowUp(t, app, "Add a docs quicklink")
	waitForTurnEnd(t, app, aiQuicklinkMade)
	modelReceived(t, fake, aiQuicklinkName)

	openRootCommand(t, app, "Snippets", "Search Snippets")
	waitForRow(t, app, aiSnippetName, elementWait)
	assertNoRow(t, app, "Rig twin")
	shoot(t, app, "ai-tools-2-snippets-command-shows-the-snippet")

	searchRootFor(t, app, aiQuicklinkKey+" hello")
	waitForRow(t, app, "Search "+aiQuicklinkName+" for “hello”", elementWait)
	shoot(t, app, "ai-tools-3-root-offers-the-quicklink")

	hidePalette(t, app)
	askFirstQuestion(t, app, "Delete the greeting snippet")
	waitForApproval(t, app, aiSnippetQuestion)
	shoot(t, app, "ai-tools-4-delete-asks")
	refuseApproval(t, app)
	waitForTurnEnd(t, app, aiSnippetKept)
	modelReceived(t, fake, "the user declined")

	askFollowUp(t, app, "Try again, I will allow it")
	waitForApproval(t, app, aiSnippetQuestion)
	allowApproval(t, app)
	waitForTurnEnd(t, app, aiSnippetDeleted)
	modelReceived(t, fake, "(the user allowed it)")

	openRootCommand(t, app, "Snippets", "Search Snippets")
	waitForRow(t, app, "Create Snippet", elementWait)
	assertNoRow(t, app, aiSnippetName)
	shoot(t, app, "ai-tools-5-snippet-deleted")
}
