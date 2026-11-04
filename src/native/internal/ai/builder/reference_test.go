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
	if apiReference != want {
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
// the Context services were cut to signatures; this keeps it from growing back past that.
const maxReferenceBytes = 48 << 10

func TestReferenceStaysWithinItsSizeBudget(t *testing.T) {
	if len(apiReference) > maxReferenceBytes {
		t.Fatalf("the reference is %d bytes; the budget is %d", len(apiReference), maxReferenceBytes)
	}
}

func TestInstructionsNameTheCurrentAPIVersion(t *testing.T) {
	if !strings.Contains(instructionsText, `"apiVersion":"1.0"`) || strings.Contains(instructionsText, "API_VERSION") {
		t.Fatal("the instructions do not carry the API version")
	}
}
