package cleanup

import (
	"fmt"
	"slices"
	"strings"
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
	got := ExtractScreenTerms(text)
	want := []string{"Kubernetes", "dashboard", "Grafana", "panel", "items", "again"}
	if !slices.Equal(got, want) {
		t.Fatalf("ExtractScreenTerms = %v, want %v", got, want)
	}
}

func TestExtractScreenTermsKeepsEdgesOnlyWhenTheyAreWordCharacters(t *testing.T) {
	got := ExtractScreenTerms("__deploy-prod__ --- 'quoted'")
	if !slices.Equal(got, []string{"deploy-prod", "quoted"}) {
		t.Fatalf("ExtractScreenTerms = %v, want the edge punctuation trimmed", got)
	}
}

func TestExtractScreenTermsKeepsEveryWord(t *testing.T) {
	var text strings.Builder
	for i := range 500 {
		fmt.Fprintf(&text, "word%dx ", i)
	}
	if got := ExtractScreenTerms(text.String()); len(got) != 500 {
		t.Fatalf("kept %d words, want all 500", len(got))
	}
}
