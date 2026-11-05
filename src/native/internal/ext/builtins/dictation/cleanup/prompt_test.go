package cleanup

import (
	"strings"
	"testing"
)

func TestPromptWithoutTermsIsTheInstructionAndTheTranscript(t *testing.T) {
	want := cleanInstruction + "\nTranscript:\nhi there"
	if got := Prompt("hi there", Terms{}); got != want {
		t.Fatalf("Prompt = %q, want %q", got, want)
	}
}

func TestPromptNamesDictionaryBeforeScreenTerms(t *testing.T) {
	got := Prompt("x", Terms{Dictionary: []string{"Nevermind"}, Screen: []string{"Figma"}})
	dictionary := strings.Index(got, "Preferred terms and spellings")
	screen := strings.Index(got, "Screen terms (words seen on screen")
	transcript := strings.Index(got, "\nTranscript:\n")
	if dictionary < 0 || screen < dictionary || transcript < screen {
		t.Fatalf("Prompt = %q, want the dictionary, then the screen terms, then the transcript", got)
	}
}

func TestPromptBudgetsTheDictionaryAndTheScreenTerms(t *testing.T) {
	long := strings.Repeat("Z", 5000)
	got := Prompt("x", Terms{Dictionary: []string{long}, Screen: []string{long}})
	if strings.Count(got, "Z") != dictionaryPromptChars+screenPromptChars {
		t.Fatalf("prompt keeps %d terms characters, want %d", strings.Count(got, "Z"), dictionaryPromptChars+screenPromptChars)
	}
}
