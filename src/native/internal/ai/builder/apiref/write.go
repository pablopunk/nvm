package apiref

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"sort"
	"strings"
)

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

// prose is doc text, or nothing in a terse index.
func (index *pkgIndex) prose(doc string) string {
	if index.terse {
		return ""
	}
	return doc
}

// functionProse is the first sentence after a function's signature, or nothing in a terse index.
func (index *pkgIndex) functionProse(doc string) string {
	if index.terse {
		return ""
	}
	return ": " + firstSentence(doc)
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
	fmt.Fprintf(b, "## %s\n\n%s\n\n", name, index.prose(index.docs[name]))
	for _, field := range iface.Methods.List {
		for _, method := range field.Names {
			fmt.Fprintf(b, "- func %s%s\n", method.Name, strings.TrimPrefix(index.exprText(field.Type), "func"))
		}
	}
	b.WriteString("\n")
}

// writeType writes one type. A full type keeps its first sentences and enum values; a signature-only type keeps the
// names and types of its members. A terse index writes no blank lines inside a type.
func (index *pkgIndex) writeType(b *strings.Builder, name string, full bool) {
	spec := index.types[name]
	fmt.Fprintf(b, "### %s\n", name)
	if !index.terse {
		b.WriteString("\n")
	}
	if doc := index.docs[name]; full && doc != "" && !index.terse {
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
	if !index.terse {
		b.WriteString("\n")
	}
}

func (index *pkgIndex) writeFields(b *strings.Builder, st *ast.StructType, full bool) {
	for _, field := range st.Fields.List {
		doc := commentText(firstNonNil(field.Doc, field.Comment))
		typeText := index.exprText(field.Type)
		if len(field.Names) == 0 {
			fmt.Fprintf(b, "- embeds %s, its fields included\n", typeText)
			continue
		}
		for _, name := range field.Names {
			line := fmt.Sprintf("- `%s` (%s)", jsonName(field.Tag, name.Name), typeText)
			if full && doc != "" && !index.terse {
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
		if value.doc != "" && !index.terse {
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
			fmt.Fprintf(b, "- %s%s%s\n", name, strings.TrimPrefix(index.exprText(decl.Type), "func"), index.functionProse(doc))
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
