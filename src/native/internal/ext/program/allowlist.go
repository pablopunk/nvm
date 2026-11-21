package program

import (
	"path"
	"reflect"
	"slices"
	"sort"
	"strings"
)

// APIImportPath is the extension api package, which every program imports as api.
const APIImportPath = "github.com/pablopunk/nvm/native/internal/ext/api"

// Packages are the standard library packages a program may use, by import path. Nothing here reaches the operating
// system: no files, processes, sockets, reflection, runtime or unsafe memory, and no goroutine or channel helper.
var Packages = []string{
	"bytes", "encoding/json", "errors", "fmt", "math", "regexp", "sort", "strconv", "strings", "time", "unicode", "unicode/utf8",
}

// deniedSymbols are exports of an allowed package that start background work or block on a channel. The interpreter
// cannot interrupt them, and a program has no goroutines of its own.
var deniedSymbols = map[string][]string{
	"time": {"After", "AfterFunc", "NewTicker", "NewTimer", "Tick"},
}

// PackageName is the name a program uses for an allowed import path: its last element.
func PackageName(importPath string) string {
	return path.Base(importPath)
}

// allowedByName maps the qualifier a program writes to the import path it stands for.
func allowedByName() map[string]string {
	byName := make(map[string]string, len(Packages))
	for _, importPath := range Packages {
		byName[PackageName(importPath)] = importPath
	}
	return byName
}

// Symbols returns the interpreter symbol table of a program: the allowed standard library packages without their denied
// exports, and the host's symbols of the api package. Every other package of either table is left out.
func Symbols(standard, host map[string]map[string]reflect.Value) map[string]map[string]reflect.Value {
	table := map[string]map[string]reflect.Value{}
	for _, importPath := range Packages {
		key := importPath + "/" + PackageName(importPath)
		exports, ok := standard[key]
		if !ok {
			continue
		}
		table[key] = withoutDenied(exports, deniedSymbols[importPath])
	}
	apiKey := APIImportPath + "/" + PackageName(APIImportPath)
	if exports, ok := host[apiKey]; ok {
		table[apiKey] = exports
	}
	return table
}

func withoutDenied(exports map[string]reflect.Value, denied []string) map[string]reflect.Value {
	kept := make(map[string]reflect.Value, len(exports))
	for name, value := range exports {
		if !slices.Contains(denied, name) {
			kept[name] = value
		}
	}
	return kept
}

// availablePackages names what a program may use, for the refusal the model reads.
func availablePackages() string {
	names := make([]string, 0, len(Packages))
	for _, importPath := range Packages {
		names = append(names, PackageName(importPath))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
