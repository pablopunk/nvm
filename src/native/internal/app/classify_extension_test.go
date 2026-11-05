package app

import (
	"testing"

	classifyimage "github.com/pablopunk/nvm/native/internal/ext/builtins/classifyimage"
	"github.com/pablopunk/nvm/native/internal/search"
)

// TestClassifyImageIsABuiltInCommand checks that the Classify Image with AI row comes from the classify built-in, and
// that its row runs the built-in's picture form.
func TestClassifyImageIsABuiltInCommand(t *testing.T) {
	p := goldenPalette(t)
	if !p.extensionManager().IsBuiltIn(classifyimage.ID) {
		t.Fatalf("%s is not a built-in", classifyimage.ID)
	}
	goldenQuery(p, "classify image")
	row := itemTitled(t, p, classifyimage.Title)
	if row.Kind != search.KindCommands || row.LearnKey != "cmd:"+classifyimage.Title {
		t.Fatalf("row = kind %q learn key %q, want a command row", row.Kind, row.LearnKey)
	}
	if len(row.Actions) == 0 || row.Actions[0].Title != classifyimage.Title {
		t.Fatalf("actions = %+v, want the row's own action", row.Actions)
	}
}

// TestClassifyImageRowIsListedAtTheRoot checks the row is offered on the root, as 1.x's AI commands extension lists it
// on every platform: nothing but the query has to match it.
func TestClassifyImageRowIsListedAtTheRoot(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, "classify image")
	for _, it := range p.Results() {
		if it.Title == classifyimage.Title {
			return
		}
	}
	t.Fatal("the classify row is not offered")
}
