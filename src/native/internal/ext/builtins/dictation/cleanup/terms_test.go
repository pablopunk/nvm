package cleanup

import (
	"slices"
	"testing"
)

func TestAWordInTheDictionaryIsADictionaryTermEvenWhenTheScreenHasIt(t *testing.T) {
	terms := NewTerms("Kubernetes\n\n  Figma  ", []string{"figma", "Grafana", "kubernetes", "Grafana"})
	if !slices.Equal(terms.Dictionary, []string{"Kubernetes", "Figma"}) {
		t.Fatalf("dictionary terms = %v, want the user's own spellings in order", terms.Dictionary)
	}
	if !slices.Equal(terms.Screen, []string{"Grafana"}) {
		t.Fatalf("screen terms = %v, want only the words the dictionary does not name", terms.Screen)
	}
}

func TestExtractScreenTermsDropsShortDigitsStopWordsAndRepeats(t *testing.T) {
	text := "The Kubernetes dashboard, 2026 and the Grafana panel. It is a db of 42 items; kubernetes again!"
	got := ExtractScreenTerms(text, 30)
	want := []string{"Kubernetes", "dashboard", "Grafana", "panel", "items", "again"}
	if !slices.Equal(got, want) {
		t.Fatalf("ExtractScreenTerms = %v, want %v", got, want)
	}
}

func TestExtractScreenTermsKeepsEdgesOnlyWhenTheyAreWordCharacters(t *testing.T) {
	got := ExtractScreenTerms("__deploy-prod__ --- 'quoted'", 30)
	if !slices.Equal(got, []string{"deploy-prod", "quoted"}) {
		t.Fatalf("ExtractScreenTerms = %v, want the edge punctuation trimmed", got)
	}
}

func TestExtractScreenTermsStopsAtTheLimit(t *testing.T) {
	got := ExtractScreenTerms("alpha bravo charlie delta echo", 2)
	if !slices.Equal(got, []string{"alpha", "bravo"}) {
		t.Fatalf("ExtractScreenTerms(limit 2) = %v", got)
	}
}

func TestScreenTermLimitFallsBackToTheDefaultForUnofferedSizes(t *testing.T) {
	if got := NormalizeScreenTermLimit(50); got != 50 {
		t.Fatalf("limit 50 = %d", got)
	}
	if got := NormalizeScreenTermLimit(7); got != DefaultScreenTermLimit {
		t.Fatalf("limit 7 = %d, want the default", got)
	}
}
