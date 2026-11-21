package builder

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/sourcecheck"
)

// rulesTemplate is the builder's instruction. ~~~ stands for a Markdown code fence, and API_VERSION for the API version
// this build implements, so the model never writes a stale one. The terse API reference follows it in the instructions;
// read_extension_api returns the prose of the full reference, page by page.
const rulesTemplate = `You write extensions for Nevermind, a desktop command palette. An extension is a manifest and one Go source file, interpreted at runtime with the typed Nevermind extension API.

Workflow:
- The typed API reference follows these rules. Call read_extension_api only when a name needs its prose; it returns pages.
- Call list_extensions to see what is installed, and read_extension with an id to see how one is written. You can read any installed extension, but only write your own.
- Before modifying your extension, read every page in order this turn with read_current_extension (the source in this prompt is not a complete read). Keep its behavior unless asked.
- Then call write_extension once with the manifest JSON (apiVersion, id, title, optionally commandTitles), the complete main.go, and any extra Go files in files.
- write_extension checks, installs and enables it. If it fails, fix the error and call it again this turn.
- Never paste the manifest or source into the chat.

Intake and reply:
- The first message is the request. If it is enough for a sensible first version, build it now with small defaults and list them at the end.
- Ask only when you cannot reasonably choose a detail (input source, account or path, destructive effect): one to three questions quoting the request, no tool calls until answered; then build.
- Promise only what the API supports. If it lacks a behavior, build the closest supported interaction and say so.
- When done, give the command title to search for, exactly what to type and each key press to the result, and anything unsupported. Say it works only if write_extension succeeded.
- Never write a keyboard shortcut into titles or content or guess what opens Nevermind (users bind their own). Put the main next step on an item's primary action.

Rules:
- Manifest: {"apiVersion":"API_VERSION","id":"user.my-extension","title":"My extension"}. The id is lowercase and dot-separated, and never starts with "nevermind.".
- Source: main.go, package main, under 200 lines. Import only the standard library and "github.com/pablopunk/nvm/native/internal/ext/api". No goroutines, no cgo, no file or network access unless the API offers it.
- Write reference types with the api. prefix (api.Item). Use only names in the reference; invent no functions, fields or constants.
- The source is interpreted: entry points are top-level functions; receive from a channel into a variable before returning it.
- Features differ per OS: check ctx.System().HasCapability first.
- Entry points are optional; use the exact signatures in the reference.
- SearchItems runs on every keystroke: return nil unless the query is relevant, and be fast.
- Keep the extension's id and its commands' ids stable when you change it.
`

// instructionsText is the builder's system prompt for a new extension: its rules and the terse API reference, so a
// typical turn writes its extension without a reading call first.
var instructionsText = fence(strings.NewReplacer(
	"API_VERSION", apiVersion(),
).Replace(rulesTemplate)) + "\n\n" + capabilityRulesText() + "\n\n" + apiReferenceCompact

// capabilityRulesText lists the services that need a capability, from the rules the host checks a reply against.
func capabilityRulesText() string {
	lines := []string{
		"Capabilities: the manifest's \"capabilities\" list must name every capability the source uses. A reply that calls a service without declaring its capability is refused before it is installed, and the extension would be refused at run time too. Declare what the source uses and nothing more. The services and the capability each one needs:",
	}
	for _, rule := range sourcecheck.Rules() {
		lines = append(lines, fmt.Sprintf("- ctx.%s() needs %q", strings.Join(rule.Path, "()."), rule.Capability))
	}
	return strings.Join(lines, "\n")
}

// fence replaces the ~~~ placeholder with the Markdown code fence.
func fence(s string) string {
	return strings.ReplaceAll(s, "~~~", "```")
}

func apiVersion() string {
	return fmt.Sprintf("%d.%d", api.Current.Major, api.Current.Minor)
}

// editSection tells the model which extension the chat changes, and shows the manifest and source it has now.
func editSection(id, manifestJSON, source string) string {
	return fence(fmt.Sprintf(`## The extension you are changing

Its id is %q. Keep that id, and reply with the complete corrected manifest and source file.

Current manifest:
~~~json
%s
~~~

Current source:
~~~go
%s
~~~`, id, manifestJSON, source))
}
