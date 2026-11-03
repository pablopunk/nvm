package design

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// internalRoot is the internal/ directory as seen from this package, where go test runs.
const internalRoot = ".."

// colorLiteralExempt are top-level directories under internal/ that may spell colors, with the reason.
var colorLiteralExempt = map[string]string{
	"design": "the tokens and their tests live here",
	"app":    "the prototype keeps its own tokens until WP0.8 moves them into design (burn down in WP0.8)",
}

var (
	hexLiteralPattern       = regexp.MustCompile(`#[0-9a-fA-F]{6}`)
	colorConstructorPattern = regexp.MustCompile(`\bui\.(Hex|RGBA?)\(`)
)

// TestColorLiteralsStayInDesignTokens fails when a non-test file outside the exempt trees spells a color:
// a six-digit hex literal, or a ui.Hex, ui.RGB or ui.RGBA call. Feature code takes colors from tokens.
func TestColorLiteralsStayInDesignTokens(t *testing.T) {
	violations, scanned, err := scanColorLiterals(internalRoot)
	if err != nil {
		t.Fatalf("scan %s: %v", internalRoot, err)
	}
	if scanned == 0 {
		t.Fatalf("scanned no Go files under %s; the test would pass vacuously", internalRoot)
	}
	for _, violation := range violations {
		t.Error(violation)
	}
}

// TestColorLiteralLintCatchesLiterals proves the detector flags each spelling and passes the rest, so a
// broken pattern cannot pass the tree scan silently.
func TestColorLiteralLintCatchesLiterals(t *testing.T) {
	tests := []struct {
		name string
		line string
		want int
	}{
		{name: "hex literal in a string", line: `var accent = "#e6bd32"`, want: 1},
		{name: "ui.Hex call", line: `c := ui.Hex("#fff")`, want: 1},
		{name: "ui.RGB call", line: `c := ui.RGB(1, 2, 3)`, want: 1},
		{name: "ui.RGBA call", line: `c := ui.RGBA(1, 2, 3, 0.5)`, want: 1},
		{name: "three-digit hex is not matched", line: `label := "#fff"`, want: 0},
		{name: "a token reference is fine", line: `ui.Box(c).Background(k.panel)`, want: 0},
		{name: "a constructor of another package is fine", line: `geom.RGB(1, 2)`, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findColorLiterals("features/example.go", []byte("package features\n\n"+tt.line+"\n"))
			if len(got) != tt.want {
				t.Fatalf("got %d violations %v, want %d", len(got), got, tt.want)
			}
		})
	}
}

// scanColorLiterals walks root and returns the violations in non-test files outside the exempt trees,
// with the number of files it examined.
func scanColorLiterals(root string) ([]string, int, error) {
	var violations []string
	scanned := 0
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		top := strings.Split(filepath.ToSlash(relative), "/")[0]
		if _, exempt := colorLiteralExempt[top]; exempt && relative != "." {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || filepath.Ext(file) != ".go" || strings.HasSuffix(file, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		scanned++
		violations = append(violations, findColorLiterals(filepath.ToSlash(file), src)...)
		return nil
	})
	return violations, scanned, err
}

// findColorLiterals reports each line of src that spells a color outside the tokens.
func findColorLiterals(filename string, src []byte) []string {
	var violations []string
	for number, line := range strings.Split(string(src), "\n") {
		if hexLiteralPattern.MatchString(line) || colorConstructorPattern.MatchString(line) {
			violations = append(violations, fmt.Sprintf("%s:%d: color literal outside internal/design tokens: %s", filename, number+1, strings.TrimSpace(line)))
		}
	}
	return violations
}
