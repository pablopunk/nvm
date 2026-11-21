package extitems

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/menubar"
)

func TestAnyLucideIconCanBeUsedInTheMenuBar(t *testing.T) {
	if !api.KnownMenuBarIcon("bird") {
		t.Fatal("a Lucide icon outside the 1.x set is not known")
	}
	if api.KnownMenuBarIcon("moonshine") {
		t.Fatal("a name that is not a Lucide icon is known")
	}
	if !api.KnownMenuBarIcon("github") {
		t.Fatal("the GitHub logo is not known")
	}
	var issues []string
	if got := normalizeIcon("bird", &issues); got != "bird" || len(issues) != 0 {
		t.Fatalf("normalizeIcon = %q, issues %v", got, issues)
	}
	if icon := menubar.MenuGlyph("bird", "darwin"); icon == nil || len(icon.PNG) == 0 {
		t.Fatal("the bird glyph is not embedded")
	}
}
