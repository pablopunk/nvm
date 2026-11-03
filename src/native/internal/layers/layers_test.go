// Package layers enforces the import order from PLAN.md section 5 over the whole module.
// It is test-only: the repository test runs `go list` and checks the import graph.
package layers

import (
	"bytes"
	"fmt"
	"os/exec"
	"path"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

const (
	modulePath = "github.com/pablopunk/nvm/native"
	appKey     = "internal/app"
	cmdKey     = "cmd/nvm"
)

// layerRank orders the tiers lowest first. A package may import packages of the same or a lower
// rank. Subpackages inherit the rank of their nearest listed ancestor.
var layerRank = map[string]int{
	"internal/platform":   0,
	"internal/store":      1,
	"internal/logx":       1,
	"internal/safego":     1,
	"internal/design":     2,
	"internal/search":     3,
	"internal/features":   4,
	"internal/backend":    5,
	"internal/ai":         6,
	"internal/ext":        6,
	"internal/jobs":       7,
	"internal/shortcuts":  7,
	"internal/windowmgmt": 7,
	"internal/dictation":  7,
	"internal/update":     7,
	"internal/diag":       7,
	"internal/palette":    8,
	appKey:                9,
	cmdKey:                10,
}

func rankOf(pkg string) (int, bool) {
	for current := pkg; current != "." && current != "/" && current != ""; current = path.Dir(current) {
		if rank, ok := layerRank[current]; ok {
			return rank, true
		}
	}
	return 0, false
}

func isUnder(pkg, root string) bool {
	return pkg == root || strings.HasPrefix(pkg, root+"/")
}

// CheckLayerViolations reports imports that point to a higher layer, and internal packages that
// import internal/app. Packages outside the layer table are ignored.
func CheckLayerViolations(graph map[string][]string) []string {
	var violations []string
	for _, importer := range sortedKeys(graph) {
		for _, imported := range graph[importer] {
			if isUnder(imported, appKey) && strings.HasPrefix(importer, "internal/") && !isUnder(importer, appKey) {
				violations = append(violations, fmt.Sprintf("%s imports %s: internal/app may only be imported by cmd/nvm", importer, imported))
				continue
			}
			importerRank, importerRanked := rankOf(importer)
			importedRank, importedRanked := rankOf(imported)
			if !importerRanked || !importedRanked {
				continue
			}
			if importedRank > importerRank {
				violations = append(violations, fmt.Sprintf("%s (rank %d) imports %s (rank %d): upward import", importer, importerRank, imported, importedRank))
			}
		}
	}
	return violations
}

// FindImportCycles returns the strongly connected components that contain a cycle, using Tarjan's
// algorithm. Only edges to packages present as keys of graph are followed.
func FindImportCycles(graph map[string][]string) [][]string {
	index := map[string]int{}
	lowLink := map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var cycles [][]string
	nextIndex := 0

	var connect func(node string)
	connect = func(node string) {
		index[node] = nextIndex
		lowLink[node] = nextIndex
		nextIndex++
		stack = append(stack, node)
		onStack[node] = true

		for _, imported := range graph[node] {
			if _, known := graph[imported]; !known {
				continue
			}
			if _, visited := index[imported]; !visited {
				connect(imported)
				lowLink[node] = min(lowLink[node], lowLink[imported])
			} else if onStack[imported] {
				lowLink[node] = min(lowLink[node], index[imported])
			}
		}

		if lowLink[node] != index[node] {
			return
		}
		var component []string
		for {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[top] = false
			component = append(component, top)
			if top == node {
				break
			}
		}
		if len(component) > 1 || slices.Contains(graph[node], node) {
			sort.Strings(component)
			cycles = append(cycles, component)
		}
	}

	for _, node := range sortedKeys(graph) {
		if _, visited := index[node]; !visited {
			connect(node)
		}
	}
	return cycles
}

func sortedKeys(graph map[string][]string) []string {
	keys := make([]string, 0, len(graph))
	for key := range graph {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func relativeToModule(importPath string) (string, bool) {
	if importPath == modulePath {
		return ".", true
	}
	if strings.HasPrefix(importPath, modulePath+"/") {
		return strings.TrimPrefix(importPath, modulePath+"/"), true
	}
	return "", false
}

// loadModuleImportGraph maps each module package (relative to the module root) to its module imports.
func loadModuleImportGraph(t *testing.T) map[string][]string {
	t.Helper()
	cmd := exec.Command("go", "list", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./...")
	cmd.Dir = "../.."
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list failed: %v\n%s", err, stderr.String())
	}

	graph := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg, inModule := relativeToModule(fields[0])
		if !inModule {
			continue
		}
		var imports []string
		for _, imported := range fields[1:] {
			if relative, ok := relativeToModule(imported); ok {
				imports = append(imports, relative)
			}
		}
		graph[pkg] = imports
	}
	return graph
}

func TestRepositoryImportsRespectLayerOrder(t *testing.T) {
	graph := loadModuleImportGraph(t)
	if len(graph) == 0 {
		t.Fatal("go list found no module packages")
	}
	for _, violation := range CheckLayerViolations(graph) {
		t.Error(violation)
	}
	for _, cycle := range FindImportCycles(graph) {
		t.Errorf("import cycle: %s", strings.Join(cycle, " -> "))
	}
}

func TestLayerCheckerFlagsViolationsAndAcceptsValidGraphs(t *testing.T) {
	tests := []struct {
		name           string
		graph          map[string][]string
		wantViolations int
	}{
		{
			name:  "downward imports pass",
			graph: map[string][]string{"internal/palette": {"internal/features"}, "internal/search": {"internal/design"}},
		},
		{
			name:  "same-rank imports pass",
			graph: map[string][]string{"internal/store": {"internal/logx"}},
		},
		{
			name:           "lower layer importing a higher layer is reported",
			graph:          map[string][]string{"internal/platform": {"internal/store"}},
			wantViolations: 1,
		},
		{
			name:           "feature importing the palette is reported",
			graph:          map[string][]string{"internal/features": {"internal/palette"}},
			wantViolations: 1,
		},
		{
			name:           "subpackage inherits its parent layer",
			graph:          map[string][]string{"internal/features/apps": {"internal/palette"}},
			wantViolations: 1,
		},
		{
			name:  "internal/app may import every ranked layer",
			graph: map[string][]string{appKey: {"internal/platform", "internal/features", "internal/palette"}},
		},
		{
			name:  "cmd/nvm may import internal/app",
			graph: map[string][]string{cmdKey: {appKey}},
		},
		{
			name:           "an internal package importing internal/app is reported",
			graph:          map[string][]string{"internal/features": {appKey}},
			wantViolations: 1,
		},
		{
			name:  "packages outside the layer table are ignored",
			graph: map[string][]string{"internal/fixtures": {"internal/palette"}, "internal/palette": {"internal/fixtures"}},
		},
		{
			name:  "unranked imports from ranked packages are ignored",
			graph: map[string][]string{"internal/store": {"internal/fixtures"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := CheckLayerViolations(tt.graph)
			if len(violations) != tt.wantViolations {
				t.Fatalf("got %d violations, want %d: %v", len(violations), tt.wantViolations, violations)
			}
		})
	}
}

func TestImportCycleCheckerFindsCycles(t *testing.T) {
	tests := []struct {
		name  string
		graph map[string][]string
		want  [][]string
	}{
		{
			name:  "acyclic graph has no cycles",
			graph: map[string][]string{"a": {"b"}, "b": {"c"}, "c": nil},
		},
		{
			name:  "three-package cycle is found",
			graph: map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}, "d": {"a"}},
			want:  [][]string{{"a", "b", "c"}},
		},
		{
			name:  "imports of unknown packages are not followed",
			graph: map[string][]string{"a": {"external"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindImportCycles(tt.graph)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got cycles %v, want %v", got, tt.want)
			}
		})
	}
}
