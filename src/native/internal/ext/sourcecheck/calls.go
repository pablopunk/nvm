package sourcecheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
)

// Use is one call in a source that needs a capability the manifest may not declare. Call is the chain as written,
// without the context variable, such as Desktop().Shell().AppleScript.
type Use struct {
	Capability api.Capability
	Call       string
}

// UsesOf returns every gated call of the source files, once per capability: the first call that needs it. The
// manifest file and test files are not source, so they are skipped.
func UsesOf(files map[string][]byte) []Use {
	var uses []Use
	seen := map[api.Capability]bool{}
	for _, file := range parseSources(files) {
		for _, chain := range callChains(file) {
			capability, ok := capabilityOf(chain.names)
			if !ok || seen[capability] {
				continue
			}
			seen[capability] = true
			uses = append(uses, Use{Capability: capability, Call: chain.text()})
		}
	}
	return uses
}

// UndeclaredUses returns the uses whose capability the manifest does not declare.
func UndeclaredUses(manifest api.Manifest, files map[string][]byte) []Use {
	declared := map[api.Capability]bool{}
	for _, capability := range manifest.Capabilities {
		declared[capability] = true
	}
	var missing []Use
	for _, use := range UsesOf(files) {
		if !declared[use.Capability] {
			missing = append(missing, use)
		}
	}
	return missing
}

// Explain says, for the model and the user, what a refused source must declare. It is "" when nothing is missing.
func Explain(missing []Use) string {
	if len(missing) == 0 {
		return ""
	}
	parts := make([]string, 0, len(missing))
	for _, use := range missing {
		parts = append(parts, fmt.Sprintf("%s needs the %q capability", use.Call, use.Capability))
	}
	return fmt.Sprintf("the source calls services that the manifest does not declare: %s. Add those capabilities to the manifest's \"capabilities\" list.", strings.Join(parts, "; "))
}

// capabilityOf returns the capability of the first rule that matches the chain, if any rule does.
func capabilityOf(names []string) (api.Capability, bool) {
	for _, rule := range rules {
		if containsRun(names, rule.Path) {
			return rule.Capability, true
		}
	}
	return "", false
}

// containsRun reports whether path appears in names as consecutive names.
func containsRun(names, path []string) bool {
	for start := 0; start+len(path) <= len(names); start++ {
		if equalNames(names[start:start+len(path)], path) {
			return true
		}
	}
	return false
}

func equalNames(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// selectorChain is the method names of one call from its context variable outward, in call order.
type selectorChain struct {
	names []string
}

func (c selectorChain) text() string {
	return "ctx." + strings.Join(c.names, "().") + "()"
}

// callChains returns the selector chain of every outermost call in the file: a call nested inside another call's
// receiver belongs to the outer chain, so it is not listed on its own.
func callChains(file *ast.File) []selectorChain {
	var chains []selectorChain
	consumed := map[ast.Node]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || consumed[call] {
			return true
		}
		chains = append(chains, selectorChain{names: chainNames(call, consumed)})
		return true
	})
	return chains
}

// chainNames walks a call's receiver chain inward and returns the names from the innermost call outward. Every node it
// passes is marked consumed, so the inner calls are not listed twice.
func chainNames(expr ast.Expr, consumed map[ast.Node]bool) []string {
	var reversed []string
	for {
		switch e := expr.(type) {
		case *ast.CallExpr:
			consumed[e] = true
			expr = e.Fun
		case *ast.SelectorExpr:
			consumed[e] = true
			reversed = append(reversed, e.Sel.Name)
			expr = e.X
		case *ast.ParenExpr:
			expr = e.X
		default:
			return reverseNames(reversed)
		}
	}
}

func reverseNames(names []string) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[len(names)-1-i] = name
	}
	return out
}

// parseSources parses each Go file of the folder that is extension source. A file that does not parse is left out:
// the child's compile check reports it with the compiler's message.
func parseSources(files map[string][]byte) []*ast.File {
	var parsed []*ast.File
	fset := token.NewFileSet()
	for name, text := range files {
		if name == bundle.ManifestFile || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, text, parser.SkipObjectResolution)
		if err == nil {
			parsed = append(parsed, file)
		}
	}
	return parsed
}
