package bundle

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// DeclaredFunctions names the top-level functions of the folder's Go source, without running it. It returns nil when a
// file cannot be read or parsed, so a caller never decides anything from a partial answer.
func DeclaredFunctions(files platform.FS, dir string) map[string]bool {
	entries, err := files.ReadDir(dir)
	if err != nil {
		return nil
	}
	declared := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := files.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, data, parser.SkipObjectResolution)
		if err != nil {
			return nil
		}
		for _, decl := range file.Decls {
			if function, ok := decl.(*ast.FuncDecl); ok && function.Recv == nil {
				declared[function.Name.Name] = true
			}
		}
	}
	return declared
}
