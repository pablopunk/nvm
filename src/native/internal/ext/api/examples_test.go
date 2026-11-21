package api

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const apiImportPath = "github.com/pablopunk/nvm/native/internal/ext/api"

// TestExampleExtensionsTypecheckAgainstTheAPI type-checks each author example with go/types. It
// proves the examples compile against this package without running any interpreter. Standard
// library imports come from the source importer; the api import is parsed from this directory.
func TestExampleExtensionsTypecheckAgainstTheAPI(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "examples", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 5 {
		t.Fatalf("found %d examples, want at least one per major shape (5)", len(files))
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := typecheckSource(file, src); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestExampleTypecheckRejectsBrokenExtensions proves the checker is not vacuous: a wrong handler
// signature and a call to a missing helper must both fail.
func TestExampleTypecheckRejectsBrokenExtensions(t *testing.T) {
	broken := map[string]string{
		"wrong search signature": `package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

type bad struct{}

var _ api.SearchProvider = bad{}

func (bad) SearchItems(query string) []api.Item { return nil }
`,
		"missing helper": `package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

var _ = api.NoSuchHelper()
`,
	}
	for name, src := range broken {
		if err := typecheckSource(name+".go", []byte(src)); err == nil {
			t.Errorf("%s: typecheck accepted a broken extension", name)
		}
	}
}

func typecheckSource(name string, src []byte) error {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, name, src, parser.AllErrors)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	imp := &apiImporter{fset: fset, std: importer.ForCompiler(fset, "source", nil)}
	config := types.Config{Importer: imp}
	_, err = config.Check("example", fset, []*ast.File{parsed}, nil)
	if err != nil {
		return fmt.Errorf("typecheck: %w", err)
	}
	return nil
}

// apiImporter resolves the api import path from the package directory and everything else from
// the standard library source importer.
type apiImporter struct {
	fset   *token.FileSet
	std    types.Importer
	loaded *types.Package
}

func (i *apiImporter) Import(path string) (*types.Package, error) {
	if path != apiImportPath {
		return i.std.Import(path)
	}
	if i.loaded != nil {
		return i.loaded, nil
	}
	files, err := parseAPISources(i.fset)
	if err != nil {
		return nil, err
	}
	config := types.Config{Importer: i}
	pkg, err := config.Check(apiImportPath, i.fset, files, nil)
	if err != nil {
		return nil, fmt.Errorf("check api package: %w", err)
	}
	i.loaded = pkg
	return pkg, nil
}

func parseAPISources(fset *token.FileSet) ([]*ast.File, error) {
	entries, err := filepath.Glob("*.go")
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, entry := range entries {
		if strings.HasSuffix(entry, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, entry, nil, parser.AllErrors)
		if err != nil {
			return nil, err
		}
		files = append(files, parsed)
	}
	return files, nil
}
