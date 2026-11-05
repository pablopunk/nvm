package builder

import (
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
// uninstall review and the alias read of the applications built-in, the API surface the builder reads.
const maxReferenceBytes = 50 << 10

func TestReferenceStaysWithinItsSizeBudget(t *testing.T) {
	if len(apiReference) > maxReferenceBytes {
		t.Fatalf("the reference is %d bytes; the budget is %d", len(apiReference), maxReferenceBytes)
	}
}

// maxCompactReferenceBytes bounds the terse reference every builder request carries. It keeps a typical turn's second
// request under the size of the request a turn used to send. It grew by about 1 KB for the uninstall review and the
// alias read of the applications built-in, which the builder reads as API surface.
const maxCompactReferenceBytes = 34 << 10

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
	if !strings.Contains(instructionsText, `"apiVersion":"1.0"`) || strings.Contains(instructionsText, "API_VERSION") {
		t.Fatal("the instructions do not carry the API version")
	}
}
