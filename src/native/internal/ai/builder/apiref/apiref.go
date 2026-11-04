// Package apiref renders the extension API reference the extension builder gives the model. It reads the typed api
// package with go/parser, so the reference is the api's own types and doc comments, and a test fails when the
// generated copy in package builder no longer matches them.
//
// Layer: ai (it imports the standard library only).
package apiref

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"sort"
	"strings"
)

// entryInterfaces are the api interfaces whose methods are the entry points an extension source defines. Each method
// name is the top-level function name the child looks up.
var entryInterfaces = []string{"CommandProvider", "RootProvider", "SearchProvider", "ActionHandler", "DataLoader", "MenuBarProvider"}

// rootTypes are the wire types the model must know beyond the entry points: the manifest, and the context the
// handlers receive. Everything they reach is included, except the services behind Context (see services).
var rootTypes = []string{"Manifest", "Context"}

// contextType is the handler argument whose accessors are the host services. Its services are rendered as names and
// signatures only, so the reference stays small while the model still sees every service it may call.
const contextType = "Context"

// Render returns the reference for the api package whose source files are at the root of sources, as plain text.
// Output is deterministic: sections and entries are sorted by name.
func Render(sources fs.FS) (string, error) {
	fset := token.NewFileSet()
	files, err := parseAPIFiles(fset, sources)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no api package source files")
	}
	index := indexPackage(fset, files)
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
		if ident, ok := node.(*ast.Ident); ok && ast.IsExported(ident.Name) {
			names = append(names, ident.Name)
		}
		return true
	})
	return names
}

// render writes the entry point section, the wire types with their docs, the Context services as signatures, and the
// api functions.
func (index *pkgIndex) render(wire, services []string) string {
	var b strings.Builder
	b.WriteString("# Extension API reference\n\n")
	b.WriteString("Entry points: an extension source defines these top-level functions, each optional, with these signatures.\n\n")
	for _, iface := range entryInterfaces {
		index.writeInterfaceMethods(&b, iface)
	}
	b.WriteString("\n## Types\n\n")
	for _, name := range wire {
		if !isEntryInterface(name) {
			index.writeType(&b, name, true)
		}
	}
	b.WriteString("## Context services\n\nThe host services a handler reaches through Context. Names and signatures only.\n\n")
	for _, name := range services {
		index.writeType(&b, name, false)
	}
	b.WriteString("## Functions\n\n")
	index.writeFunctions(&b)
	return b.String()
}

func isEntryInterface(name string) bool {
	for _, iface := range entryInterfaces {
		if iface == name {
			return true
		}
	}
	return false
}

func (index *pkgIndex) writeInterfaceMethods(b *strings.Builder, name string) {
	spec, ok := index.types[name]
	if !ok {
		return
	}
	iface, ok := spec.Type.(*ast.InterfaceType)
	if !ok {
		return
	}
	fmt.Fprintf(b, "## %s\n\n%s\n\n", name, index.docs[name])
	for _, field := range iface.Methods.List {
		for _, method := range field.Names {
			fmt.Fprintf(b, "- func %s%s\n", method.Name, strings.TrimPrefix(index.exprText(field.Type), "func"))
		}
	}
	b.WriteString("\n")
}

// writeType writes one type. A full type keeps its first sentences and enum values; a signature-only type keeps the
// names and types of its members.
func (index *pkgIndex) writeType(b *strings.Builder, name string, full bool) {
	spec := index.types[name]
	fmt.Fprintf(b, "### %s\n\n", name)
	if doc := index.docs[name]; full && doc != "" {
		fmt.Fprintf(b, "%s\n\n", firstSentence(doc))
	}
	switch t := spec.Type.(type) {
	case *ast.StructType:
		index.writeFields(b, t, full)
	case *ast.InterfaceType:
		for _, field := range t.Methods.List {
			for _, method := range field.Names {
				fmt.Fprintf(b, "- %s%s\n", method.Name, strings.TrimPrefix(index.exprText(field.Type), "func"))
			}
		}
	default:
		fmt.Fprintf(b, "Type: %s.\n", index.exprText(spec.Type))
		if full {
			index.writeConsts(b, name)
		}
	}
	b.WriteString("\n")
}

func (index *pkgIndex) writeFields(b *strings.Builder, st *ast.StructType, full bool) {
	for _, field := range st.Fields.List {
		doc := commentText(firstNonNil(field.Doc, field.Comment))
		typeText := index.exprText(field.Type)
		if len(field.Names) == 0 {
			fmt.Fprintf(b, "- embeds %s: its fields are part of this type\n", typeText)
			continue
		}
		for _, name := range field.Names {
			line := fmt.Sprintf("- `%s` (%s)", jsonName(field.Tag, name.Name), typeText)
			if full && doc != "" {
				line += ": " + firstSentence(doc)
			}
			b.WriteString(line + "\n")
		}
	}
}

// writeConsts lists the values of an enum type, such as ActionKind, in declaration order.
func (index *pkgIndex) writeConsts(b *strings.Builder, typeName string) {
	values := index.consts[typeName]
	if len(values) == 0 {
		return
	}
	b.WriteString("Values:\n")
	for _, value := range values {
		line := fmt.Sprintf("- %s = %s", value.name, value.value)
		if value.doc != "" {
			line += ": " + value.doc
		}
		b.WriteString(line + "\n")
	}
}

func (index *pkgIndex) writeFunctions(b *strings.Builder) {
	names := make([]string, 0, len(index.funcs))
	for name := range index.funcs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		decl := index.funcs[name]
		if doc := commentText(decl.Doc); doc != "" {
			fmt.Fprintf(b, "- %s%s: %s\n", name, strings.TrimPrefix(index.exprText(decl.Type), "func"), firstSentence(doc))
		}
	}
}

// exprText prints an expression as Go source.
func (index *pkgIndex) exprText(node ast.Node) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, index.fset, node)
	return buf.String()
}

// jsonName returns the wire name of a field: its json tag's name, or the Go name when the tag has none.
func jsonName(tag *ast.BasicLit, goName string) string {
	if tag == nil {
		return goName
	}
	raw := strings.Trim(tag.Value, "`")
	const key = `json:"`
	start := strings.Index(raw, key)
	if start < 0 {
		return goName
	}
	value := raw[start+len(key):]
	value = value[:strings.Index(value, `"`)]
	name, _, _ := strings.Cut(value, ",")
	if name == "" || name == "-" {
		return goName
	}
	return name
}

// commentText joins a comment group into one line of prose.
func commentText(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return strings.Join(strings.Fields(group.Text()), " ")
}

// firstSentence keeps the first sentence of a doc comment, which says what the thing is.
func firstSentence(doc string) string {
	if before, _, found := strings.Cut(doc, ". "); found {
		return before + "."
	}
	return doc
}

func firstNonNil(groups ...*ast.CommentGroup) *ast.CommentGroup {
	for _, group := range groups {
		if group != nil {
			return group
		}
	}
	return nil
}
