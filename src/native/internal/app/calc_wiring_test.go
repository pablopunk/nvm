package app

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
)

// calcQuery is the percentage-and-power query of the J03 journey: 36 + 96.
const calcQuery = "15% of 240 + 3*(2^5)"

// TestCalculatorRowShowsTheResultOfAPercentageQuery checks that the root shows the calculator's row for the
// canned query, with the 1.x title and the result copied by Copy Result.
func TestCalculatorRowShowsTheResultOfAPercentageQuery(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, calcQuery)
	row := itemTitled(t, p, "15% of 240 + 3*(2^5) = 132")
	if row.Kind != search.KindCalculator {
		t.Fatalf("kind = %q, want the calculator", row.Kind)
	}
	if row.Subtitle != "Copy result to clipboard" {
		t.Fatalf("subtitle = %q", row.Subtitle)
	}
}

// TestCalculatorCopyResultWritesTheResultToTheClipboard runs the first action of the row, which Enter runs, and
// reads the clipboard back.
func TestCalculatorCopyResultWritesTheResultToTheClipboard(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, calcQuery)
	row := itemTitled(t, p, "15% of 240 + 3*(2^5) = 132")
	row.Actions[0].Run(p)
	got, err := p.ports.Clipboard.ReadText()
	if err != nil {
		t.Fatal(err)
	}
	if got != "132" {
		t.Fatalf("clipboard = %q, want 132", got)
	}
}

// TestCalculatorRowOffersTheOneActionsOf1x checks the action order of the 1.x extension: Copy Result first, then
// Continue Calculation, Copy Unformatted Result only when formatting changed the value, and Paste Result last.
func TestCalculatorRowOffersTheActionsOf1x(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, calcQuery)
	row := itemTitled(t, p, "15% of 240 + 3*(2^5) = 132")
	var titles []string
	for _, a := range row.Actions {
		titles = append(titles, a.Title)
	}
	want := []string{"Copy Result", "Continue Calculation", "Paste Result"}
	if strings.Join(titles, "|") != strings.Join(want, "|") {
		t.Fatalf("actions = %v, want %v", titles, want)
	}
}
