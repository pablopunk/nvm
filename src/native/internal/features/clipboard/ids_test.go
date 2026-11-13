package clipboard

import (
	"strings"
	"testing"
)

// TestTextIDIsStable ports 'clipboardHistoryIdForText produces stable id': the same text always gets the same
// ID, and surrounding whitespace does not change it.
func TestTextIDIsStable(t *testing.T) {
	first := TextID("hello world")
	if !strings.HasPrefix(first, "text:") {
		t.Fatalf("TextID = %q, want a text: prefix", first)
	}
	if first != TextID("hello world") {
		t.Fatal("the same text got two IDs")
	}
	if first != TextID("  hello world \n") {
		t.Fatal("surrounding whitespace changed the ID")
	}
}

// TestTextIDIsEmptyForBlankText ports 'clipboardHistoryIdForText returns empty for empty text'.
func TestTextIDIsEmptyForBlankText(t *testing.T) {
	if TextID("") != "" || TextID("  ") != "" {
		t.Fatal("blank text has an ID")
	}
}
