package apps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/sourcecheck"
)

// TestEveryServiceTheSourceCallsIsDeclared reads the built-in's own source and checks each gated call (Desktop().Files()
// for the path labels of the review, for one) against the manifest, as the builder checks an extension's source.
func TestEveryServiceTheSourceCallsIsDeclared(t *testing.T) {
	files := packageSources(t)
	if missing := sourcecheck.UndeclaredUses(Extension{}.Manifest(), files); len(missing) > 0 {
		t.Fatalf("the manifest lacks capabilities its source calls:\n%s", sourcecheck.Explain(missing))
	}
}

// packageSources returns the non-test Go files of this package, keyed by file name.
func packageSources(t *testing.T) map[string][]byte {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = data
	}
	if len(files) == 0 {
		t.Fatal("no sources found")
	}
	return files
}
