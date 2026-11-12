// Package apiref renders the extension API reference the extension builder gives the model. It reads the typed api
// package with go/parser, so the reference is the api's own types and doc comments, and a test fails when the
// generated copy in package builder no longer matches them.
//
// Layer: ai (it imports the standard library only).
package apiref

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"sort"
	"strings"
)

// entryInterfaces are the api interfaces whose methods are the entry points an extension source defines. Each method
// name is the top-level function name the child looks up.
var entryInterfaces = []string{"CommandProvider", "RootProvider", "SearchProvider", "ActionHandler", "DataLoader", "MenuBarProvider", "WindowRestorer"}

// rootTypes are the wire types the model must know beyond the entry points: the manifest, and the context the
// handlers receive. Everything they reach is included, except the services behind Context (see services).
var rootTypes = []string{"Manifest", "Context"}

// contextType is the handler argument whose accessors are the host services. Its services are rendered as names and
// signatures only, so the reference stays small while the model still sees every service it may call.
const contextType = "Context"

// Render returns the reference for the api package whose source files are at the root of sources, as plain text.
// Output is deterministic: sections and entries are sorted by name.
func Render(sources fs.FS) (string, error) {
	return render(sources, false)
}

// RenderCompact returns the reference without its prose: the same entry points, types, enum values and functions as
// Render, with names and signatures only. The builder preloads it, so a typical turn needs no reading tool call.
func RenderCompact(sources fs.FS) (string, error) {
	return render(sources, true)
}

// RenderSubset returns the terse reference of only the listed Context accessors, such as "Notes", and the types they
// reach. A program that uses a few services reads this instead of the whole reference. An accessor the Context does not
// declare is an error, so a rename in the api cannot silently empty the text.
func RenderSubset(sources fs.FS, accessors []string) (string, error) {
	fset := token.NewFileSet()
	files, err := parseAPIFiles(fset, sources)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no api package source files")
	}
	index := indexPackage(fset, files)
	index.terse = true
	return index.renderSubset(accessors)
}

func render(sources fs.FS, terse bool) (string, error) {
	fset := token.NewFileSet()
	files, err := parseAPIFiles(fset, sources)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no api package source files")
	}
	index := indexPackage(fset, files)
	index.terse = terse
	wire := index.reach(append(append([]string{}, entryInterfaces...), rootTypes...), map[string]bool{contextType: true})
	return index.render(wire, index.services(wire)), nil
}

// parseAPIFiles parses the non-test Go files of package api at the root of sources, in name order.
func parseAPIFiles(fset *token.FileSet, sources fs.FS) ([]*ast.File, error) {
	entries, err := fs.ReadDir(sources, ".")
	if err != nil {
		return nil, fmt.Errorf("read api sources: %w", err)
	}
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := fs.ReadFile(sources, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		if file.Name.Name == "api" {
			files = append(files, file)
		}
	}
	return files, nil
}

// pkgIndex is the package's declarations by name.
type pkgIndex struct {
	fset   *token.FileSet
	types  map[string]*ast.TypeSpec
	docs   map[string]string
	funcs  map[string]*ast.FuncDecl
	consts map[string][]constValue
	// terse drops the prose: doc sentences of types, fields, values, functions and interfaces.
	terse bool
}

type constValue struct {
	name  string
	value string
	doc   string
}

func indexPackage(fset *token.FileSet, files []*ast.File) *pkgIndex {
	index := &pkgIndex{fset: fset, types: map[string]*ast.TypeSpec{}, docs: map[string]string{}, funcs: map[string]*ast.FuncDecl{}, consts: map[string][]constValue{}}
	for _, file := range files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				index.addGenDecl(d)
			case *ast.FuncDecl:
				if d.Recv == nil && ast.IsExported(d.Name.Name) {
					index.funcs[d.Name.Name] = d
				}
			}
		}
	}
	return index
}

func (index *pkgIndex) addGenDecl(decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if !ast.IsExported(s.Name.Name) {
				continue
			}
			index.types[s.Name.Name] = s
			index.docs[s.Name.Name] = commentText(firstNonNil(s.Doc, decl.Doc))
		case *ast.ValueSpec:
			index.addConstSpec(decl, s)
		}
	}
}

// addConstSpec records the exported constants of a typed constant block, keyed by their type name, so an enum prints
// with its values.
func (index *pkgIndex) addConstSpec(decl *ast.GenDecl, spec *ast.ValueSpec) {
	if decl.Tok != token.CONST {
		return
	}
	typeIdent, ok := spec.Type.(*ast.Ident)
	if !ok {
		return
	}
	for i, name := range spec.Names {
		if !ast.IsExported(name.Name) || i >= len(spec.Values) {
			continue
		}
		value := index.exprText(spec.Values[i])
		doc := commentText(firstNonNil(spec.Doc, spec.Comment))
		index.consts[typeIdent.Name] = append(index.consts[typeIdent.Name], constValue{name: name.Name, value: value, doc: doc})
	}
}

// services are the types the Context reaches that the wire types do not reach.
func (index *pkgIndex) services(wire []string) []string {
	known := map[string]bool{}
	for _, name := range wire {
		known[name] = true
	}
	var services []string
	for _, name := range index.reach(index.referencedTypes(contextType), known) {
		if _, declared := index.types[name]; declared && !known[name] {
			services = append(services, name)
		}
	}
	return services
}

// reach returns the exported type names reachable from roots, following field, method and embedded types. A name in
// stop is reached but its own references are not followed.
func (index *pkgIndex) reach(roots []string, stop map[string]bool) []string {
	seen := map[string]bool{}
	queue := append([]string(nil), roots...)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] {
			continue
		}
		seen[name] = true
		if stop[name] {
			continue
		}
		for _, next := range index.referencedTypes(name) {
			if _, declared := index.types[next]; declared && !seen[next] {
				queue = append(queue, next)
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// referencedTypes lists the exported identifiers a declaration's type expression names.
func (index *pkgIndex) referencedTypes(name string) []string {
	spec, ok := index.types[name]
	if !ok {
		return nil
	}
	var names []string
	ast.Inspect(spec.Type, func(node ast.Node) bool {
		if field, ok := node.(*ast.Field); ok && hiddenField(field) {
			return false
		}
		if ident, ok := node.(*ast.Ident); ok && ast.IsExported(ident.Name) {
			names = append(names, ident.Name)
		}
		return true
	})
	return names
}
