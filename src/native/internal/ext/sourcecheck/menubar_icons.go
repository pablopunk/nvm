package sourcecheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// UnknownMenuBarIcons lists the menu bar icons the source names literally that the host cannot draw. The host would show
// the default bolt instead, so whoever wrote the source hears about it now. A name computed at run time is not seen.
func UnknownMenuBarIcons(files map[string][]byte) []string {
	var unknown []string
	for _, file := range parseSources(files) {
		ast.Inspect(file, func(node ast.Node) bool {
			for _, name := range menuBarIconLiterals(node) {
				if !api.KnownMenuBarIcon(api.MenuBarIcon(name)) && !slices.Contains(unknown, name) {
					unknown = append(unknown, name)
				}
			}
			return true
		})
	}
	slices.Sort(unknown)
	return unknown
}

// ExplainUnknownIcons is the refusal the author reads.
func ExplainUnknownIcons(unknown []string) string {
	return fmt.Sprintf("%s: not a menu bar icon. Use a Lucide icon name in kebab-case, or a logo such as github.", strings.Join(unknown, ", "))
}

// menuBarIconLiterals reads the icon names one node spells out: api.MenuBarIcon("x"), or Icon: "x" in a menu bar or row.
func menuBarIconLiterals(node ast.Node) []string {
	switch n := node.(type) {
	case *ast.CallExpr:
		if isAPIType(n.Fun, "MenuBarIcon") && len(n.Args) == 1 {
			return stringLiteral(n.Args[0])
		}
	case *ast.CompositeLit:
		if isAPIType(n.Type, "MenuBar") || isAPIType(n.Type, "MenuRow") {
			return iconField(n)
		}
	}
	return nil
}

func iconField(lit *ast.CompositeLit) []string {
	for _, element := range lit.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := field.Key.(*ast.Ident); ok && key.Name == "Icon" {
			return stringLiteral(field.Value)
		}
	}
	return nil
}

func isAPIType(expr ast.Expr, name string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "api"
}

func stringLiteral(expr ast.Expr) []string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return nil
	}
	name, err := strconv.Unquote(lit.Value)
	if err != nil {
		return nil
	}
	return []string{name}
}
