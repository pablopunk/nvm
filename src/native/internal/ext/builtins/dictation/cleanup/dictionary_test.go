package cleanup

import (
	"slices"
	"testing"
)

func TestAddTermKeepsTheDictionaryOneTermPerLineWithoutRepeats(t *testing.T) {
	dictionary, changed := AddTerm("Figma", "  Kubernetes ")
	if !changed || dictionary != "Figma\nKubernetes" {
		t.Fatalf("AddTerm = %q, %v; want the term appended", dictionary, changed)
	}
	if again, changed := AddTerm(dictionary, "figma"); changed || again != dictionary {
		t.Fatalf("AddTerm of a repeat = %q, %v; want no change", again, changed)
	}
}

func TestAddTermRefusesBlankAndMultiLineTerms(t *testing.T) {
	for _, term := range []string{"", "   ", "two\nlines"} {
		if got, changed := AddTerm("Figma", term); changed || got != "Figma" {
			t.Fatalf("AddTerm(%q) = %q, %v; want no change", term, got, changed)
		}
	}
}

func TestRemoveTermDropsEveryLineThatNamesIt(t *testing.T) {
	got := RemoveTerm("Figma\nfigma\nKubernetes", "FIGMA")
	if got != "Kubernetes" {
		t.Fatalf("RemoveTerm = %q, want only Kubernetes", got)
	}
	if !slices.Equal(DictionaryTerms("a\n\n b "), []string{"a", "b"}) {
		t.Fatalf("DictionaryTerms drops blank lines and trims")
	}
}
