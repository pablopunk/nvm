package store

import (
	"strings"
	"testing"
	"time"
)

// largeBudget is the save budget for a five megabyte document in the fake file system.
const largeBudget = 200 * time.Millisecond

// blobDoc is a document dominated by one large string, as a clipboard history or a chat log would be.
type blobDoc struct {
	Blob string `json:"blob"`
}

func TestFiveMegabyteDocumentSavesWithinBudget(t *testing.T) {
	s, _, _ := newFixture(t)
	doc := Doc(s, "blob.json", 1, func() blobDoc { return blobDoc{} })
	blob := strings.Repeat("x", 5<<20)
	if err := doc.Update(func(v *blobDoc) { v.Blob = blob }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	start := time.Now()
	if err := doc.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	elapsed := time.Since(start)
	t.Logf("saved 5 MB in %v (race detector: %v)", elapsed, raceEnabled)
	if !raceEnabled && elapsed > largeBudget {
		t.Fatalf("saving 5 MB took %v, budget %v", elapsed, largeBudget)
	}
}
