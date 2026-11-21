package diag

import (
	"regexp"
	"testing"
)

var referenceShape = regexp.MustCompile(`^NVM-[ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{5}$`)

func TestReferencesHaveTheDocumentedShapeAndNeverRepeat(t *testing.T) {
	seen := map[string]bool{}
	for range 5000 {
		ref := NewReference()
		if !referenceShape.MatchString(ref) {
			t.Fatalf("reference %q does not match NVM-XXXXX", ref)
		}
		if seen[ref] {
			t.Fatalf("reference %q issued twice in one run", ref)
		}
		seen[ref] = true
	}
}

// TestReferenceSourceSkipsIssuedReferences drives the collision rule with a scripted generator that
// repeats itself, so the second draw of a value must be thrown away.
func TestReferenceSourceSkipsIssuedReferences(t *testing.T) {
	draws := []string{"NVM-AAAAA", "NVM-AAAAA", "NVM-AAAAA", "NVM-BBBBB"}
	next := 0
	source := newReferenceSource(func() string {
		value := draws[next]
		next++
		return value
	})
	if got := source.next(); got != "NVM-AAAAA" {
		t.Fatalf("first reference = %q", got)
	}
	if got := source.next(); got != "NVM-BBBBB" {
		t.Fatalf("second reference = %q, want the first unused draw", got)
	}
}
