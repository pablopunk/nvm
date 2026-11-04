package child

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAuthorExamplesLoadAsInterpretedExtensions evaluates each author example the way the child loads an extension.
// The examples are checked by go/types in the api package; this test checks the runtime path: the interpreter finds
// an entry point with the signature the protocol expects.
func TestAuthorExamplesLoadAsInterpretedExtensions(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "api", "testdata", "examples", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 5 {
		t.Fatalf("found %d examples, want at least 5", len(files))
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			i, err := evalExtension(string(source))
			if err != nil {
				t.Fatalf("does not load: %v", err)
			}
			functions := lookupEntryPoints(i)
			if len(functions) == 0 {
				t.Fatal("declares no top-level entry point")
			}
			if err := checkEntryPointSignatures(functions); err != nil {
				t.Fatalf("entry point signatures: %v", err)
			}
		})
	}
}
