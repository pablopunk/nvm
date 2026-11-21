package apiref

import (
	"os"
	"strings"
	"testing"
)

func TestCompactPreservesReferenceSignatures(t *testing.T) {
	fs := os.DirFS("../../../ext/api")
	full, err := Render(fs)
	if err != nil {
		t.Fatal(err)
	}
	compact, err := RenderCompact(fs)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{
		{"- `handle` (string)", "handle:string"},
		{"- `items` ([]MenuItem)", "items:[]MenuItem"},
		{"- CapabilityDesktopMenus = \"desktop.menus\"", "- CapabilityDesktopMenus = \"desktop.menus\""},
		{"- Snapshot() (MenuSnapshot, error)", "- Snapshot() (MenuSnapshot, error)"},
		{"- func SearchItems(ctx Context, query string) ([]Item, error)", "- func SearchItems(ctx Context, query string) ([]Item, error)"},
	} {
		if !strings.Contains(full, pair[0]) || !strings.Contains(compact, pair[1]) {
			t.Errorf("missing reference entry %q / %q", pair[0], pair[1])
		}
	}
}

func TestReferenceDescriptionPreservesMeaningWithoutRepeatingItsIdentifier(t *testing.T) {
	if got := referenceDescription("Action", "Action is one declarative operation. More guidance."); got != "One declarative operation." {
		t.Fatal(got)
	}
	if got := referenceDescription("CopyText", "CopyText writes text."); got != "Writes text." {
		t.Fatal(got)
	}
	if got := referenceDescription("Name", "Use this carefully."); got != "Use this carefully." {
		t.Fatal(got)
	}
}

// TestReferenceListsPlatformAvailability: the model learns which operations are limited to an OS, in the full and the
// compact reference.
func TestReferenceListsPlatformAvailability(t *testing.T) {
	fs := os.DirFS("../../../ext/api")
	full, err := Render(fs)
	if err != nil {
		t.Fatal(err)
	}
	compact, err := RenderCompact(fs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(full, "## Platform availability") || !strings.Contains(full, "Desktop().Shell().AppleScript: macOS only.") {
		t.Error("the full reference does not say AppleScript is macOS only")
	}
	if !strings.Contains(compact, "macOS only: Shell().AppleScript") {
		t.Error("the compact reference does not say AppleScript is macOS only")
	}
}
