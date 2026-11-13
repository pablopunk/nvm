package services

import (
	"regexp"
	"slices"
	"strings"
)

// templatePattern matches {name} and {{name}}. Names cannot contain braces, so the match never nests.
var templatePattern = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}|\{\s*([^{}]+?)\s*\}`)

const (
	calculatorPrefix = "calculator:"
	argumentPrefix   = "argument:"
)

// expandTemplate fills the placeholders of input from values. {calculator:expr} is evaluated with calculate, and an
// unanswered expression becomes empty text. {argument:name} reads a value the same way {name} does. A placeholder
// with no value is empty and listed in missing. The cursor is the byte offset of the first cursorToken, which is
// removed from the text; nil when the template has none.
func expandTemplate(input string, values map[string]string, cursorToken string, calculate func(string) (string, bool)) (string, *int, []string) {
	var missing []string
	expanded := templatePattern.ReplaceAllStringFunc(input, func(match string) string {
		name := strings.TrimSpace(strings.Trim(match, "{} "))
		switch {
		case strings.HasPrefix(name, calculatorPrefix):
			result, _ := calculate(strings.TrimSpace(strings.TrimPrefix(name, calculatorPrefix)))
			return result
		case strings.HasPrefix(name, argumentPrefix):
			return lookupPlaceholder(values, strings.TrimSpace(strings.TrimPrefix(name, argumentPrefix)), &missing)
		}
		return lookupPlaceholder(values, name, &missing)
	})
	cursor := indexOfCursor(expanded, cursorToken)
	text := expanded
	if cursor >= 0 {
		text = strings.Replace(expanded, cursorToken, "", 1)
	}
	return text, cursorPointer(cursor), uniqueSorted(missing)
}

func lookupPlaceholder(values map[string]string, name string, missing *[]string) string {
	value, ok := values[name]
	if !ok {
		*missing = append(*missing, name)
	}
	return value
}

func indexOfCursor(text, cursorToken string) int {
	if cursorToken == "" {
		return -1
	}
	return strings.Index(text, cursorToken)
}

func cursorPointer(cursor int) *int {
	if cursor < 0 {
		return nil
	}
	return &cursor
}

func uniqueSorted(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	sorted := slices.Clone(names)
	slices.Sort(sorted)
	return slices.Compact(sorted)
}
