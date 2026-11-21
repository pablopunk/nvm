package backend

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// apiDirEnv overrides where the backend's pages/api tree is read. The default is the checked-out repository. The
// shared test box has no repository root, so boxgo copies the tree there and points this variable at it.
const apiDirEnv = "NVM_BACKEND_API_DIR"

const defaultBackendAPIDir = "../../../backend/src/pages/api"

// clientRouteSources are the packages whose /api/ constants name the routes the client and its fake backend use.
var clientRouteSources = []string{".", "../fixtures"}

// routeConstant is one /api/ string constant and the name it is declared under.
type routeConstant struct {
	name  string
	route string
}

// TestEveryRouteTheClientCallsExistsInTheBackend checks that each route constant maps to an Astro API file in the
// backend. A route the backend does not serve fails the build here, before a desktop release can call it.
func TestEveryRouteTheClientCallsExistsInTheBackend(t *testing.T) {
	apiDir := backendAPIDir(t)
	routes := clientRouteConstants(t)
	if len(routes) < 10 {
		t.Fatalf("found %d route constants, want the client's routes; the scan lost them", len(routes))
	}
	for _, rc := range routes {
		if !servedByPagesRoute(apiDir, rc.route) {
			t.Errorf("%s = %q has no file under %s, so the backend does not serve it", rc.name, rc.route, apiDir)
		}
	}
}

// TestCatchAllFileDoesNotProveItsPathIsServed checks the resolver against a catch-all: a [...path] file answers many
// paths, so it must not satisfy a literal route such as /api/v1/models.
func TestCatchAllFileDoesNotProveItsPathIsServed(t *testing.T) {
	dir := t.TempDir()
	writeRouteFile(t, filepath.Join(dir, "v1", "models", "[...path].ts"))
	writeRouteFile(t, filepath.Join(dir, "v1", "active-model.ts"))
	writeRouteFile(t, filepath.Join(dir, "tokens", "index.ts"))

	if servedByPagesRoute(dir, "/api/v1/models") {
		t.Fatal("/api/v1/models is served only by a catch-all, which must not count")
	}
	if !servedByPagesRoute(dir, "/api/v1/active-model") {
		t.Fatal("/api/v1/active-model has a file, but the resolver did not find it")
	}
	if !servedByPagesRoute(dir, "/api/tokens") {
		t.Fatal("/api/tokens is served by index.ts, but the resolver did not find it")
	}
}

// backendAPIDir returns the backend's pages/api directory and fails when it is missing, so the check never passes
// silently on a tree without the backend.
func backendAPIDir(t *testing.T) string {
	t.Helper()
	dir := defaultBackendAPIDir
	if override := os.Getenv(apiDirEnv); override != "" {
		dir = override
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("backend pages/api not found at %s (set %s to its path): %v", dir, apiDirEnv, err)
	}
	return dir
}

// clientRouteConstants collects every /api/ string constant declared in the client and fixture packages, sorted by
// route. Test files are excluded: they name routes the backend may not serve in order to test error handling.
func clientRouteConstants(t *testing.T) []routeConstant {
	t.Helper()
	var found []routeConstant
	for _, dir := range clientRouteSources {
		for _, file := range parseNonTestFiles(t, dir) {
			found = append(found, apiConstantsIn(file)...)
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].route < found[j].route })
	return found
}

// parseNonTestFiles parses the non-test Go files of dir.
func parseNonTestFiles(t *testing.T, dir string) []*ast.File {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.Mode(0))
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, file)
	}
	return files
}

// apiConstantsIn returns the /api/ string constants declared at the top level of one file.
func apiConstantsIn(file *ast.File) []routeConstant {
	var found []routeConstant
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range value.Names {
				if i >= len(value.Values) {
					continue
				}
				if route, ok := apiRouteLiteral(value.Values[i]); ok {
					found = append(found, routeConstant{name: name.Name, route: route})
				}
			}
		}
	}
	return found
}

// apiRouteLiteral reports the value of a string literal that starts with /api/.
func apiRouteLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	route, err := strconv.Unquote(lit.Value)
	if err != nil || !strings.HasPrefix(route, "/api/") {
		return "", false
	}
	return route, true
}

// servedByPagesRoute reports whether apiDir has a file that answers route by its literal path: route.ts or route.js,
// or an index file in a directory of that name. A dynamic segment such as [id] or [...path] never counts.
func servedByPagesRoute(apiDir, route string) bool {
	segments := strings.Split(strings.TrimPrefix(route, "/api/"), "/")
	for _, segment := range segments {
		if segment == "" || strings.ContainsAny(segment, "[]") {
			return false
		}
	}
	base := filepath.Join(append([]string{apiDir}, segments...)...)
	candidates := []string{base + ".ts", base + ".js", filepath.Join(base, "index.ts"), filepath.Join(base, "index.js")}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

// writeRouteFile creates an empty route file and its directories.
func writeRouteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}
