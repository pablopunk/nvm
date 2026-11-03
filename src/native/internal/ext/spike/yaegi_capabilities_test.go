//go:build spike

package spike

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/traefik/yaegi/stdlib"
)

// symbolCoveragePackages are the packages extensions are most likely to use.
var symbolCoveragePackages = []string{
	"bufio", "bytes", "cmp", "context", "encoding/base64", "encoding/binary", "encoding/csv",
	"encoding/hex", "encoding/json", "errors", "fmt", "io", "io/fs", "iter", "log", "log/slog",
	"maps", "math", "math/bits", "math/rand", "math/rand/v2", "net/url", "os", "os/exec", "path",
	"path/filepath", "reflect", "regexp", "runtime", "slices", "sort", "strconv", "strings", "sync",
	"sync/atomic", "text/template", "time", "unicode", "unicode/utf8", "unique", "structs",
}

// TestYaegiSymbolCoverageAgainstGoroot compares the exported API of each GOROOT package (for the
// linux/amd64 build context) with the Yaegi symbol table. It reports missing names, which is the
// Go-version-bump risk: Yaegi only ships symbol tables generated for older Go releases.
func TestYaegiSymbolCoverageAgainstGoroot(t *testing.T) {
	var totalExported, totalPresent int
	for _, pkg := range symbolCoveragePackages {
		exported, err := exportedNamesOf(pkg)
		requireNoError(t, err)
		present := yaegiNamesOf(pkg)
		var missing []string
		for name := range exported {
			if !present[name] {
				missing = append(missing, name)
			}
		}
		slices.Sort(missing)
		totalExported += len(exported)
		totalPresent += len(exported) - len(missing)
		shown := missing
		if len(shown) > 8 {
			shown = shown[:8]
		}
		measureLine(t, "symbols pkg=%s exported=%d present=%d missing=%d e.g.=%v",
			pkg, len(exported), len(exported)-len(missing), len(missing), shown)
	}
	measureLine(t, "symbols total exported=%d present=%d coverage_pct=%.1f",
		totalExported, totalPresent, 100*float64(totalPresent)/float64(max(totalExported, 1)))
}

// TestYaegiMissingStdlibPackages lists GOROOT packages (outside internal, cmd and vendor) that the
// Yaegi symbol table does not provide at all.
func TestYaegiMissingStdlibPackages(t *testing.T) {
	provided := map[string]bool{}
	for key := range stdlib.Symbols {
		provided[path.Dir(key)] = true
	}
	all, err := goRootPackages()
	requireNoError(t, err)
	var missing []string
	for _, pkg := range all {
		if !provided[pkg] {
			missing = append(missing, pkg)
		}
	}
	slices.Sort(missing)
	measureLine(t, "stdlib goroot_packages=%d yaegi_provided=%d missing=%d", len(all), len(all)-len(missing), len(missing))
	measureLine(t, "stdlib missing_list=%v", missing)
}

// TestYaegiGenericPrograms runs five small generic programs through the interpreter.
func TestYaegiGenericPrograms(t *testing.T) {
	for _, program := range GenericsPrograms {
		got, err := evalValue(program.Src, "Run()")
		switch {
		case err != nil:
			measureLine(t, "generics program=%s status=FAIL reason=%q", program.Name, firstLine(err.Error()))
		case got != program.Want:
			measureLine(t, "generics program=%s status=WRONG got=%q want=%q", program.Name, got, program.Want)
		default:
			measureLine(t, "generics program=%s status=ok", program.Name)
		}
	}
}

// TestYaegiGoroutinesAndChannels checks that interpreted goroutines and channels compute the
// right result (plain receives, no select).
func TestYaegiGoroutinesAndChannels(t *testing.T) {
	source := `package main

func Run() int {
	ch := make(chan int)
	done := make(chan int)
	go func() {
		sum := 0
		for v := range ch {
			sum += v
		}
		done <- sum
	}()
	for i := 1; i <= 100; i++ {
		ch <- i
	}
	close(ch)
	return <-done
}
`
	got, err := evalValue(source, "Run()")
	measureLine(t, "goroutines plain-recv sum_1_to_100=%s err=%q", got, firstLine(errString(err)))
}

// TestYaegiGoroutineShapes narrows down which goroutine and channel shape fails, one program each.
func TestYaegiGoroutineShapes(t *testing.T) {
	shapes := []struct {
		name   string
		source string
	}{
		{"goroutine sends, main receives into variable", `package main
func Run() int {
	ch := make(chan int)
	go func() { ch <- 5 }()
	x := <-ch
	return x
}
`},
		{"buffered, receive as return expression", `package main
func Run() int {
	ch := make(chan int, 1)
	ch <- 5
	return <-ch
}
`},
		{"goroutine sends, main receives as return expression", `package main
func Run() int {
	ch := make(chan int)
	go func() { ch <- 5 }()
	return <-ch
}
`},
		{"range over buffered channel", `package main
func Run() int {
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	close(ch)
	s := 0
	for v := range ch {
		s += v
	}
	return s
}
`},
		{"goroutine ranges and reports", `package main
func Run() int {
	ch := make(chan int)
	done := make(chan int)
	go func() {
		s := 0
		for v := range ch {
			s += v
		}
		done <- s
	}()
	ch <- 1
	ch <- 2
	close(ch)
	return <-done
}
`},
		{"closure mutates captured variable", `package main
func Run() int {
	n := 0
	func() { n += 2 }()
	return n
}
`},
		{"goroutine assigns captured variable via channel", `package main
func Run() int {
	var total int
	done := make(chan bool)
	go func() {
		total = 9
		done <- true
	}()
	<-done
	return total
}
`},
	}
	for _, shape := range shapes {
		got, err := evalValue(shape.source, "Run()")
		measureLine(t, "goroutine-shape %q got=%q err=%q", shape.name, got, firstLine(errString(err)))
	}
}

// TestYaegiSelectWithDefinedCase isolates a select whose case defines a variable. It is the shape
// most extension code will use for timeouts and cancellation.
func TestYaegiSelectWithDefinedCase(t *testing.T) {
	cases := map[string]string{
		"select case with :=": `package main

func Run() int {
	done := make(chan int, 1)
	done <- 7
	select {
	case total := <-done:
		return total
	default:
		return 0
	}
}
`,
		"select case with plain receive": `package main

func Run() int {
	done := make(chan int, 1)
	done <- 7
	var total int
	select {
	case total = <-done:
		return total
	default:
		return 0
	}
}
`,
	}
	for _, name := range []string{"select case with :=", "select case with plain receive"} {
		got, err := evalValue(cases[name], "Run()")
		measureLine(t, "select %q got=%s err=%q", name, got, firstLine(errString(err)))
	}
}

// TestYaegiStdlibSelectors separates "the import succeeds" from "the selector is callable". A
// package can import through the source fallback while none of its exported names resolve.
func TestYaegiStdlibSelectors(t *testing.T) {
	cases := []struct {
		name   string
		source string
		expr   string
	}{
		{"strings.ToUpper (symbol table)", "package main\n\nimport \"strings\"\n\nfunc Run() string { return strings.ToUpper(\"a\") }\n", "Run()"},
		{"slices.Index (generic)", "package main\n\nimport \"slices\"\n\nfunc Run() int { return slices.Index([]int{4, 5}, 5) }\n", "Run()"},
		{"cmp.Compare (generic)", "package main\n\nimport \"cmp\"\n\nfunc Run() int { return cmp.Compare(1, 2) }\n", "Run()"},
		{"maps.Keys (generic iterator)", "package main\n\nimport \"maps\"\n\nfunc Run() int { return len(maps.Keys(map[string]int{\"a\": 1})) }\n", "Run()"},
		{"os/exec LookPath (restricted)", "package main\n\nimport \"os/exec\"\n\nfunc Run() string { p, _ := exec.LookPath(\"sh\"); return p }\n", "Run()"},
	}
	for _, c := range cases {
		got, err := evalValue(c.source, c.expr)
		measureLine(t, "selector %q got=%q err=%q", c.name, got, firstLine(errString(err)))
	}
	got, err := evalUnrestricted("package main\n\nimport \"os/exec\"\n\nfunc Run() string { p, _ := exec.LookPath(\"sh\"); return p }\n", "Run()")
	measureLine(t, "selector %q got=%q err=%q", "os/exec LookPath (unrestricted option)", got, firstLine(errString(err)))
}

// TestYaegiPanicsSurfaceAsErrors checks what a panic inside interpreted code looks like to the
// host when the call is made on the host goroutine.
func TestYaegiPanicsSurfaceAsErrors(t *testing.T) {
	cases := map[string]string{
		"explicit panic": `package main
func Run() int { panic("boom") }
`,
		"index out of range": `package main
func Run() int { xs := []int{}; return xs[3] }
`,
		"nil map write": `package main
func Run() int { var m map[string]int; m["a"] = 1; return 1 }
`,
	}
	for _, name := range []string{"explicit panic", "index out of range", "nil map write"} {
		_, err := evalValue(cases[name], "Run()")
		if err == nil {
			measureLine(t, "panic case=%q surfaced=no", name)
			continue
		}
		measureLine(t, "panic case=%q surfaced=error host_survived=yes message=%q", name, firstLine(err.Error()))
	}
}

// exportedNamesOf returns the exported top-level names of a GOROOT package for the linux/amd64
// build context, skipping tests. Methods are not counted because Yaegi exposes them through types.
func exportedNamesOf(pkg string) (map[string]bool, error) {
	dir := filepath.Join(build.Default.GOROOT, "src", filepath.FromSlash(pkg))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	names := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if matched, _ := build.Default.MatchFile(dir, name); !matched {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		collectExported(file, names)
	}
	return names, nil
}

func collectExported(file *ast.File, names map[string]bool) {
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Recv == nil && decl.Name.IsExported() {
				names[decl.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if spec.Name.IsExported() {
						names[spec.Name.Name] = true
					}
				case *ast.ValueSpec:
					for _, ident := range spec.Names {
						if ident.IsExported() {
							names[ident.Name] = true
						}
					}
				}
			}
		}
	}
}

// yaegiNamesOf returns the names the Yaegi table exposes for an import path.
func yaegiNamesOf(pkg string) map[string]bool {
	names := map[string]bool{}
	for key, values := range stdlib.Symbols {
		if path.Dir(key) != pkg {
			continue
		}
		for name := range values {
			names[name] = true
		}
	}
	return names
}

// goRootPackages lists importable GOROOT packages that have non-test Go files for linux/amd64.
func goRootPackages() ([]string, error) {
	root := filepath.Join(build.Default.GOROOT, "src")
	var pkgs []string
	err := filepath.WalkDir(root, func(file string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		switch entry.Name() {
		case "cmd", "vendor", "internal", "testdata":
			if file != root {
				return filepath.SkipDir
			}
		}
		if hasBuildableGoFile(file) {
			rel, err := filepath.Rel(root, file)
			if err != nil {
				return err
			}
			pkgs = append(pkgs, filepath.ToSlash(rel))
		}
		return nil
	})
	return pkgs, err
}

func hasBuildableGoFile(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if matched, _ := build.Default.MatchFile(dir, name); matched {
			return true
		}
	}
	return false
}
