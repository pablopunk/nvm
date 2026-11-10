package child

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// TestAFolderOfSeveralGoFilesLoadsAsOneExtension: main.go calls a helper that lives in another file of the folder, and
// that file imports a package main.go does not.
func TestAFolderOfSeveralGoFilesLoadsAsOneExtension(t *testing.T) {
	dir := t.TempDir()
	writeSourceFile(t, dir, "main.go", "package main\n\nimport \"github.com/pablopunk/nvm/native/internal/ext/api\"\n\nfunc RootItems(ctx api.Context) ([]api.Item, error) {\n\treturn []api.Item{{ID: \"a\", Title: shout(\"hi\")}}, nil\n}\n")
	writeSourceFile(t, dir, "helpers.go", "package main\n\nimport (\n\t\"strings\"\n\n\tapi \"github.com/pablopunk/nvm/native/internal/ext/api\"\n)\n\nvar _ api.Item\n\nfunc shout(s string) string { return strings.ToUpper(s) }\n")
	writeSourceFile(t, dir, "helpers_test.go", "package main\n\nthis is not go\n")
	source, err := loadSource(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatalf("loadSource: %v", err)
	}
	if strings.Count(source, "package main") != 1 || strings.Count(source, "\"strings\"") != 1 {
		t.Fatalf("merged source has one package clause and one strings import; got:\n%s", source)
	}
	i, err := evalExtension(source)
	if err != nil {
		t.Fatalf("evalExtension: %v", err)
	}
	if _, ok := lookupEntryPoints(i)[protocol.MethodRoot]; !ok {
		t.Fatal("the merged extension has no root entry point")
	}
}

func writeSourceFile(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
