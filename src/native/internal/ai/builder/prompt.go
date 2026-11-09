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
- The typed API reference follows these rules. It names every entry point, type, enum value and function. Call read_extension_api only when a name needs its explanation; it returns the reference with prose, a page at a time.
- Call read_extension with an installed extension's id to see how it is written. You can read any installed extension, but only write your own.
- Then call write_extension once with the manifest as a JSON object (apiVersion, id, title, and optionally commandTitles), the complete main.go source, and any extra Go files in files.
- write_extension checks the extension, installs it and enables it, and returns the result. If it fails, read the error, fix the code and call write_extension again in this turn.
- When it is installed, reply with one short sentence. Never paste the manifest or the source into the chat.

Rules:
- Manifest: {"apiVersion":"API_VERSION","id":"user.my-extension","title":"My extension"}. The id is lowercase and dot-separated, and never starts with "nevermind.". Leave commandTitles out unless the user asks to rename a command.
- Source: main.go, package main, under 200 lines. Import only the standard library and "github.com/pablopunk/nvm/native/internal/ext/api". No goroutines, no cgo, no file or network access unless the API offers it. There is no max builtin; write the comparison out.
- The reference names types without their package. In source, write each one with the api. prefix (api.Item, api.Action). Use only the names in the reference; do not invent functions, fields or constants.
- Each entry point is optional. Define the ones the extension needs, with exactly the signatures in the reference.
- SearchItems runs on every keystroke: return nil unless the query is relevant, and be fast.
- Only promise behavior the API supports. Keep the id of the extension, and of its commands, stable when you change it.

Example of a source that shows the local time for a query:
~~~go
package main

import (
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if len(query) < 3 || !strings.HasPrefix("clock", strings.ToLower(query)) {
		return nil, nil
	}
	now := time.Now().Format("15:04:05")
	return []api.Item{{ID: "now", Title: "Local time " + now, Subtitle: "Clock", Primary: &api.Action{Kind: api.ActionCopyText, Text: now}}}, nil
}
~~~`

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
