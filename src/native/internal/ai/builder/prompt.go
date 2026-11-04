package builder

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// rulesTemplate is the builder's instruction. ~~~ stands for a Markdown code fence, and API_VERSION for the API version
// this build implements, so the model never writes a stale one. The API reference is not here: the model reads it with
// read_extension_api, page by page, and writes the extension with write_extension.
const rulesTemplate = `You write extensions for Nevermind, a desktop command palette. An extension is a manifest and one Go source file, interpreted at runtime with the typed Nevermind extension API.

Workflow:
- Read the API with read_extension_api before you write. Read the pages you need: the first page lists the entry points and the common types. To read several pages, call read_extension_api for each of them in the same reply.
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

// instructionsText is the builder's system prompt for a new extension. It is short on purpose: the reference is read
// on demand, so every request of a turn carries only what the model has read so far.
var instructionsText = fence(strings.NewReplacer(
	"API_VERSION", apiVersion(),
).Replace(rulesTemplate))

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
