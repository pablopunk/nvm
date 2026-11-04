package platform

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

// TestAllCapabilitiesListsEveryDeclaredCapability parses this package's Capability constants and checks that
// AllCapabilities names each one, so a capability added on one branch cannot be dropped from the list by a merge.
func TestAllCapabilitiesListsEveryDeclaredCapability(t *testing.T) {
	declared := declaredCapabilityValues(t)
	listed := map[Capability]bool{}
	for _, c := range AllCapabilities() {
		listed[c] = true
	}
	for _, value := range declared {
		if !listed[Capability(value)] {
			t.Errorf("capability %q is declared but missing from AllCapabilities", value)
		}
	}
	if len(listed) != len(declared) {
		t.Errorf("AllCapabilities has %d entries, %d capabilities are declared", len(listed), len(declared))
	}
}

// declaredCapabilityValues returns the string value of every constant of type Capability in capabilities.go.
func declaredCapabilityValues(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "capabilities.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || spec.Type == nil {
			return true
		}
		if ident, ok := spec.Type.(*ast.Ident); !ok || ident.Name != "Capability" {
			return true
		}
		for _, v := range spec.Values {
			if lit, ok := v.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				values = append(values, strings.Trim(lit.Value, `"`))
			}
		}
		return true
	})
	slices.Sort(values)
	return values
}
