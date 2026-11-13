package fixtures

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

// TestExtensionSourcesParseAsExtensions checks each fixture is a Go file that declares package main and an
// Extension function, so later work packages can feed them to the interpreter without a syntax surprise.
func TestExtensionSourcesParseAsExtensions(t *testing.T) {
	sources := ExtensionSources()
	if len(sources) != 6 {
		t.Fatalf("got %d extension fixtures, want 6", len(sources))
	}
	for _, fixture := range sources {
		t.Run(fixture.Name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), fixture.Name+".go", fixture.Source, parser.ParseComments)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if file.Name.Name != "main" {
				t.Fatalf("package %q, want main", file.Name.Name)
			}
			if !declaresFunction(file, "Extension") {
				t.Fatal("no Extension function")
			}
		})
	}
}

// TestHostileExtensionReachesForProcessesAndCredentials documents what the hostile fixture does, so a
// sandbox test later has a known target, and checks that the good fixture stays within the host API.
func TestHostileExtensionReachesForProcessesAndCredentials(t *testing.T) {
	imports := importsOf(t, ExtensionSources(), "hostile")
	if !imports["os/exec"] || !imports["os"] {
		t.Fatalf("hostile imports = %v; it must import os and os/exec", imports)
	}
	goodImports := importsOf(t, ExtensionSources(), "good")
	for path := range goodImports {
		if path != "nvm" {
			t.Fatalf("good fixture imports %q; it may only use the nvm host package", path)
		}
	}
}

// declaresFunction reports whether the file has a top-level function with the given name.
func declaresFunction(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return true
		}
	}
	return false
}

// importsOf returns the set of import paths of the named fixture.
func importsOf(t *testing.T, sources []ExtensionFixture, name string) map[string]bool {
	t.Helper()
	for _, fixture := range sources {
		if fixture.Name != name {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name+".go", fixture.Source, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		imports := map[string]bool{}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			imports[path] = true
		}
		return imports
	}
	t.Fatalf("no fixture named %q", name)
	return nil
}
