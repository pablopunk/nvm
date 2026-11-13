package platform

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// internalRoot is the internal/ directory as seen from this package, where go test runs.
const internalRoot = ".."

// exemptTrees are top-level directories under internal/ that the scan skips entirely, with the reason.
// The ports and their implementations are the only code allowed to touch the OS and network directly.
var exemptTrees = map[string]string{
	"platform": "the ports and their implementations are the only code allowed to touch the OS",
	"app":      "the prototype; burn down in later work packages (PLAN.md section 5)",
	"rigtest":  "rig test tooling drives the real OS from tests and is never imported by production code",
}

// allowedImports are forbidden imports that a top-level directory may still use, with the reason.
var allowedImports = map[string]map[string]string{
	"fixtures": {
		"net/http": "the fake backend is an http.Handler; fixtures are test and dev data, never production code",
	},
	"design": {
		"github.com/egoist/mygo/ui":          "the golden harness renders views with ui.NewTester; it is test tooling that no production code imports",
		"github.com/egoist/mygo/ui/richtext": "the Markdown editor draws with MyGo's rich text editor; richtext holds no window or OS service",
	},
	"perf": {
		"net/http": "the counters endpoint is served only in -tags perfdebug builds on a loopback address; default builds link no listener",
	},
	"search": {
		"github.com/egoist/mygo/ui": "an Item carries a ui.Bitmap and a Detail closure that draws with ui.Context; the import is ui only, never mygo itself",
	},
	"palette": {
		"github.com/egoist/mygo/ui":          "the palette shell builds ui views and lists (ui.View, ui.ListState, ui.Handle); the import is ui only, the window and OS services stay in app",
		"github.com/egoist/mygo/ui/markdown": "extension views draw Markdown with the same renderer as chat replies; the import is ui/markdown only and holds no window or OS service",
	},
}

// forbiddenImports are packages feature code must reach only through a port. Each entry also matches its
// sub-packages.
var forbiddenImports = []string{"os/exec", "net/http", "github.com/egoist/mygo"}

// forbiddenCalls are package-qualified functions feature code must not call; they read the wall clock.
var forbiddenCalls = map[string]string{"time": "Now"}

// TestNonPlatformPackagesStayOffTheOSAndNetwork fails when feature code under internal/ imports os/exec or
// net/http, imports MyGo, or reads the wall clock with time.Now. Such code must receive a port instead.
func TestNonPlatformPackagesStayOffTheOSAndNetwork(t *testing.T) {
	violations, scanned, err := scanInternalTree(internalRoot)
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

// TestPortRuleCatchesViolations proves the detector finds each kind of violation, and only those, over
// in-memory sources. Without it a broken detector would pass the tree scan for the wrong reason.
func TestPortRuleCatchesViolations(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		allowed map[string]string
		want    int
	}{
		{
			name: "os/exec import",
			src:  "package features\n\nimport \"os/exec\"\n\nvar _ = exec.Command\n",
			want: 1,
		},
		{
			name: "aliased os/exec import",
			src:  "package features\n\nimport ex \"os/exec\"\n\nvar _ = ex.Command\n",
			want: 1,
		},
		{
			name: "net/http import",
			src:  "package features\n\nimport \"net/http\"\n\nvar _ = http.Get\n",
			want: 1,
		},
		{
			name: "MyGo root import",
			src:  "package features\n\nimport \"github.com/egoist/mygo\"\n\nvar _ = mygo.App\n",
			want: 1,
		},
		{
			name: "MyGo sub-package import",
			src:  "package features\n\nimport \"github.com/egoist/mygo/ui\"\n\nvar _ = ui.Context{}\n",
			want: 1,
		},
		{
			name: "time.Now call",
			src:  "package features\n\nimport \"time\"\n\nvar _ = time.Now()\n",
			want: 1,
		},
		{
			name: "time.Now through an aliased time import",
			src:  "package features\n\nimport tm \"time\"\n\nvar _ = tm.Now()\n",
			want: 1,
		},
		{
			name: "time.Since and time.Date are allowed",
			src:  "package features\n\nimport \"time\"\n\nvar _ = time.Since(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))\n",
			want: 0,
		},
		{
			name: "mentions in comments and raw strings are not imports",
			src:  "package features\n\n// uses os/exec and time.Now() in docs only\nvar source = `import \"os/exec\"; time.Now()`\n",
			want: 0,
		},
		{
			name:    "an allowed import is not reported",
			src:     "package fixtures\n\nimport \"net/http\"\n\nvar _ = http.StatusOK\n",
			allowed: allowedImports["fixtures"],
			want:    0,
		},
		{
			name:    "an allowance does not cover the clock",
			src:     "package fixtures\n\nimport (\n\t\"net/http\"\n\t\"time\"\n)\n\nvar _ = http.StatusOK\nvar _ = time.Now()\n",
			allowed: allowedImports["fixtures"],
			want:    1,
		},
		{
			name: "a local variable named time is not the clock",
			src:  "package features\n\ntype clock struct{}\n\nfunc (clock) Now() int { return 0 }\n\nvar time = clock{}\nvar _ = time.Now()\n",
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findPortViolations("features/example.go", []byte(tt.src), tt.allowed)
			if err != nil {
				t.Fatalf("findPortViolations: %v", err)
			}
			if len(got) != tt.want {
				t.Fatalf("got %d violations %v, want %d", len(got), got, tt.want)
			}
		})
	}
}

// scanInternalTree checks every non-test Go file under root, skipping exempt trees. It returns the
// violations and how many files it examined.
func scanInternalTree(root string) ([]string, int, error) {
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
		if entry.IsDir() {
			if _, exempt := exemptTrees[top]; exempt && relative != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(file) != ".go" || strings.HasSuffix(file, "_test.go") {
			return nil
		}
		if _, exempt := exemptTrees[top]; exempt {
			return nil
		}
		src, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		scanned++
		found, err := findPortViolations(filepath.ToSlash(file), src, allowedImports[top])
		if err != nil {
			return err
		}
		violations = append(violations, found...)
		return nil
	})
	return violations, scanned, err
}

// findPortViolations parses one file and reports forbidden imports that are not allowed, and forbidden calls.
// Comments and string literals never match, because the check works on the syntax tree.
func findPortViolations(filename string, src []byte, allowed map[string]string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}
	var violations []string
	localNames := map[string]string{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		localNames[importPath] = localName(spec, importPath)
		if !isForbiddenImport(importPath) {
			continue
		}
		if _, ok := allowed[importPath]; ok {
			continue
		}
		violations = append(violations, fmt.Sprintf("%s: imports %q; use a platform port instead", position(fset, spec.Pos()), importPath))
	}

	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		for importPath, funcName := range forbiddenCalls {
			if localNames[importPath] == receiver.Name && selector.Sel.Name == funcName {
				violations = append(violations, fmt.Sprintf("%s: calls %s.%s; take a platform.Clock instead", position(fset, call.Pos()), receiver.Name, funcName))
			}
		}
		return true
	})
	return violations, nil
}

// isForbiddenImport reports whether importPath is one of the forbidden packages or inside one of them.
func isForbiddenImport(importPath string) bool {
	for _, forbidden := range forbiddenImports {
		if importPath == forbidden || strings.HasPrefix(importPath, forbidden+"/") {
			return true
		}
	}
	return false
}

// localName returns the identifier an import is referred to by in the file.
func localName(spec *ast.ImportSpec, importPath string) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	if i := strings.LastIndex(importPath, "/"); i >= 0 {
		return importPath[i+1:]
	}
	return importPath
}

// position formats a token position as file:line for a violation message.
func position(fset *token.FileSet, pos token.Pos) string {
	p := fset.Position(pos)
	return fmt.Sprintf("%s:%d", p.Filename, p.Line)
}
