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
		{"- `handle` (string)", "- handle: string"},
		{"- `items` ([]MenuItem)", "- items: []MenuItem"},
		{"- CapabilityDesktopMenus = \"desktop.menus\"", "- CapabilityDesktopMenus = \"desktop.menus\""},
		{"- Snapshot() (MenuSnapshot, error)", "- Snapshot() (MenuSnapshot, error)"},
		{"- func SearchItems(ctx Context, query string) ([]Item, error)", "- func SearchItems(ctx Context, query string) ([]Item, error)"},
	} {
		if !strings.Contains(full, pair[0]) || !strings.Contains(compact, pair[1]) {
			t.Errorf("missing reference entry %q / %q", pair[0], pair[1])
		}
	}
}
