package sourcecheck

import (
	"go/ast"
	"slices"
	"strconv"
	"strings"
)

// DisplayTitles returns the sorted, distinct titles of the commands and root items the source declares, as literals of
// api.Command and api.Item. A title the source builds at runtime is not found; the list is a best reading of the text.
func DisplayTitles(files map[string][]byte) []string {
	seen := map[string]bool{}
	var titles []string
	for _, file := range parseSources(files) {
		ast.Inspect(file, func(node ast.Node) bool {
			for _, title := range titlesOfLiteral(node) {
				key := normalizeTitle(title)
				if key != "" && !seen[key] {
					seen[key] = true
					titles = append(titles, strings.TrimSpace(title))
				}
			}
			return true
		})
	}
	slices.Sort(titles)
	return titles
}

// titlesOfLiteral returns the titles of an api.Command or api.Item literal, or of the elided literals of a slice of them.
func titlesOfLiteral(node ast.Node) []string {
	lit, ok := node.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	switch typed := lit.Type.(type) {
	case *ast.SelectorExpr:
		if isDisplayType(typed) {
			return titleOf(lit)
		}
	case *ast.ArrayType:
		if selector, ok := typed.Elt.(*ast.SelectorExpr); ok && isDisplayType(selector) {
			return titlesOfElements(lit)
		}
	}
	return nil
}

func titlesOfElements(lit *ast.CompositeLit) []string {
	var titles []string
	for _, element := range lit.Elts {
		if inner, ok := element.(*ast.CompositeLit); ok && inner.Type == nil {
			titles = append(titles, titleOf(inner)...)
		}
	}
	return titles
}

// isDisplayType reports whether a selector names api.Command or api.Item.
func isDisplayType(selector *ast.SelectorExpr) bool {
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "api" && (selector.Sel.Name == "Command" || selector.Sel.Name == "Item")
}

// titleOf returns the Title field of a literal when it is a string constant.
func titleOf(lit *ast.CompositeLit) []string {
	for _, element := range lit.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.Ident)
		if !ok || key.Name != "Title" {
			continue
		}
		value, ok := pair.Value.(*ast.BasicLit)
		if !ok {
			return nil
		}
		text, err := strconv.Unquote(value.Value)
		if err != nil {
			return nil
		}
		return []string{text}
	}
	return nil
}

// normalizeTitle is the form two titles are compared in: case and surrounding space do not make a title different.
func normalizeTitle(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(title), " "))
}

// SameTitle reports whether two titles name the same row, as the duplicate check compares them.
func SameTitle(a, b string) bool {
	return normalizeTitle(a) != "" && normalizeTitle(a) == normalizeTitle(b)
}
