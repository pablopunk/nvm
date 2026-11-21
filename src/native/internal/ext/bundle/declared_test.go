package bundle

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestDeclaredFunctionsReadsEveryFileAndIgnoresMethods(t *testing.T) {
	files := fake.NewFS()
	if err := files.MkdirAll("/ext", 0o755); err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{
		"main.go":      "package main\nfunc Commands() {}\ntype T struct{}\nfunc (T) SearchItems() {}\n",
		"other.go":     "package main\nfunc RunAction() {}\n",
		"skip_test.go": "package main\nfunc RootItems() {}\n",
	}
	for name, source := range sources {
		if err := files.WriteFile("/ext/"+name, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := DeclaredFunctions(files, "/ext")
	if !got["Commands"] || !got["RunAction"] || got["SearchItems"] || got["RootItems"] {
		t.Fatalf("got %v", got)
	}
	if err := files.WriteFile("/ext/bad.go", []byte("package main\nfunc ("), 0o644); err != nil {
		t.Fatal(err)
	}
	if DeclaredFunctions(files, "/ext") != nil {
		t.Fatal("a file that does not parse must give no answer")
	}
}
