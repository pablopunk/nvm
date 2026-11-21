package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// unwiredManagerFields are the manager inputs the app leaves unset on purpose. Entries can only be removed.
var unwiredManagerFields = map[string]string{
	"Options.Budgets":     "the supervisor's DefaultBudgets apply",
	"Options.MemoryLimit": "an address-space cap breaks Go's large reservations; ResidentLimit is the real guard",
}

func structFields(t *testing.T, path, name string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fields []string
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok || spec.Name.Name != name {
			return true
		}
		for _, field := range spec.Type.(*ast.StructType).Fields.List {
			for _, ident := range field.Names {
				fields = append(fields, ident.Name)
			}
		}
		return false
	})
	return fields
}

func literalKeys(t *testing.T, path, selector string) map[string]bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if !ok {
			return true
		}
		named, ok := literal.Type.(*ast.SelectorExpr)
		if !ok || named.Sel.Name != selector {
			return true
		}
		for _, element := range literal.Elts {
			if pair, ok := element.(*ast.KeyValueExpr); ok {
				keys[pair.Key.(*ast.Ident).Name] = true
			}
		}
		return true
	})
	return keys
}

// TestAppWiresEveryExtensionServicePort is part of gate G4 of docs/extension-api-parity-plan.md: a port the app forgets
// makes its service answer "not available in this build" in the shipped app, which no unit test with fakes would catch.
func TestAppWiresEveryExtensionServicePort(t *testing.T) {
	for _, structure := range []string{"Ports", "Options"} {
		keys := literalKeys(t, "extensions_host.go", structure)
		for _, field := range structFields(t, "../ext/manager/manager.go", structure) {
			qualified := structure + "." + field
			if _, allowed := unwiredManagerFields[qualified]; !keys[field] && !allowed && !strings.HasPrefix(field, "test") {
				t.Errorf("the app does not set manager.%s", qualified)
			}
		}
	}
}
