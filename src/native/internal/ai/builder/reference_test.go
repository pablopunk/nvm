package builder

import (
	"fmt"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"os"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/builder/apiref"
)

// TestGeneratedReferenceMatchesTheAPI fails when the api package changed and go generate was not run, so the model's
// reference cannot drift from the typed API.
func TestGeneratedReferenceMatchesTheAPI(t *testing.T) {
	want, err := apiref.Render(os.DirFS("../../ext/api"))
	if err != nil {
		t.Fatal(err)
	}
	wantCompact, err := apiref.RenderCompact(os.DirFS("../../ext/api"))
	if err != nil {
		t.Fatal(err)
	}
	if apiReference != want || apiReferenceCompact != wantCompact {
		t.Fatal("reference_gen.go is stale: run go generate in internal/ai/builder")
	}
}

func TestReferenceNamesTheEntryPointsAndTheirContext(t *testing.T) {
	for _, want := range []string{"func SearchItems(ctx Context, query string) ([]Item, error)", "`primary`", "ActionCopyText = \"copy-text\"", "Storage"} {
		if !strings.Contains(apiReference, want) {
			t.Errorf("the reference lacks %q", want)
		}
	}
}

// maxReferenceBytes bounds the reference the model reads on every builder turn. The reference was 64 KB of text before
// the Context services were cut to signatures; this keeps it from growing back past that. It grew by about 1 KB for the
// uninstall review and the alias read of the applications built-in, and by about 1 KB more for the bundled datasets, the
// view search and the grid categories. It grew again for the files built-in's footer, Tab kind key, indexing empty state,
// file detail and index status, the API surface the builder reads. The settings views (section 37 of the extension API
// doc) and the Background Tasks jobs namespace (section 41) grew it again. The Notes built-in and the extension window
// chrome (section 44) grew it by about 1 KB more.
// The AI Chats built-in's chat and builder API, merged over the shared displays, grew it to about 61 KB. Closing the
// remaining 1.x API gaps (Self, Selection.Read) took it past that.
const maxReferenceBytes = 62 << 10

func TestReferenceStaysWithinItsSizeBudget(t *testing.T) {
	if len(apiReference) > maxReferenceBytes {
		t.Fatalf("the reference is %d bytes; the budget is %d", len(apiReference), maxReferenceBytes)
	}
}

// maxCompactReferenceBytes bounds the terse reference every builder request carries. It keeps a typical turn's second
// request under the size of the request a turn used to send. It grew by about 1 KB for the uninstall review and the
// alias read of the applications built-in, which the builder reads as API surface. It grew by about 1.2 KB for the
// files built-in's preview, open-with and trash actions, its index status and the bundled datasets, the API surface the
// built-ins moved onto. The diagnostics built-in's records and report and the jobs namespace grew it again. The Notes
// built-in's list, editor and window actions grew it by about 0.8 KB more. The AI Chats built-in's chat and builder
// actions grew it to about 39 KB.
const maxCompactReferenceBytes = 39 << 10

func TestCompactReferenceStaysWithinItsSizeBudget(t *testing.T) {
	if len(apiReferenceCompact) > maxCompactReferenceBytes {
		t.Fatalf("the compact reference is %d bytes; the budget is %d", len(apiReferenceCompact), maxCompactReferenceBytes)
	}
}

func TestInstructionsCarryTheCompactReference(t *testing.T) {
	if !strings.HasSuffix(instructionsText, apiReferenceCompact) {
		t.Fatal("the instructions do not end with the compact reference")
	}
}

func TestInstructionsNameTheCurrentAPIVersion(t *testing.T) {
	if !strings.Contains(instructionsText, fmt.Sprintf(`"apiVersion":%q`, api.Current.String())) || strings.Contains(instructionsText, "API_VERSION") {
		t.Fatal("the instructions do not carry the API version")
	}
}
