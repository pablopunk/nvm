package child

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// loadSource returns the source of the extension folder that holds the file at path. A folder with one file is that
// file's text. A folder with several .go files is their merge into one source, because the interpreter cannot import
// the same package twice across evaluations.
func loadSource(path string) (string, error) {
	files, err := extensionSourceFiles(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	if len(files) <= 1 {
		source, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read extension source: %w", err)
		}
		return string(source), nil
	}
	return mergeSources(files)
}

// extensionSourceFiles lists the .go files of a folder, sorted, without test files.
func extensionSourceFiles(dir string) (map[string][]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read extension folder: %w", err)
	}
	files := map[string][]byte{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		files[name] = data
	}
	return files, nil
}

// mergeSources joins package main files into one source: the union of their imports, then each file's declarations in
// name order. Each file's package clause and import declarations are removed from its text.
func mergeSources(files map[string][]byte) (string, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	imports := importSet{}
	var bodies bytes.Buffer
	for _, name := range names {
		body, fileImports, err := splitFile(name, files[name])
		if err != nil {
			return "", err
		}
		for _, spec := range fileImports {
			imports.add(spec)
		}
		bodies.Write(body)
		bodies.WriteString("\n")
	}
	specs := imports.specs()
	var merged strings.Builder
	merged.WriteString("package main\n\nimport (\n")
	for _, spec := range specs {
		merged.WriteString("\t" + spec + "\n")
	}
	merged.WriteString(")\n\n")
	merged.Write(bodies.Bytes())
	return merged.String(), nil
}

// splitFile returns the declarations of one file, without its package clause and imports, and the import specs it
// declares as written (with any alias).
func splitFile(name string, data []byte) ([]byte, []string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, data, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("extension source %s does not parse: %w", name, err)
	}
	var cuts [][2]int
	var specs []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		cuts = append(cuts, [2]int{fset.Position(gen.Pos()).Offset, fset.Position(gen.End()).Offset})
		for _, spec := range gen.Specs {
			specs = append(specs, importSpecText(spec.(*ast.ImportSpec)))
		}
	}
	cuts = append(cuts, [2]int{0, fset.Position(file.Name.End()).Offset})
	var body bytes.Buffer
	last := 0
	sort.Slice(cuts, func(i, j int) bool { return cuts[i][0] < cuts[j][0] })
	for _, cut := range cuts {
		body.Write(data[last:cut[0]])
		last = cut[1]
	}
	body.Write(data[last:])
	return body.Bytes(), specs, nil
}

// importSpecText writes one import spec as the source has it: an alias and then the quoted path.
func importSpecText(spec *ast.ImportSpec) string {
	if spec.Name != nil {
		return spec.Name.Name + " " + spec.Path.Value
	}
	return spec.Path.Value
}

// importSet keeps one import spec per path. An import the source names without an alias wins over an aliased one of
// the same path, because the interpreter binds one name per path and the default name is the one the source uses.
type importSet map[string]string

func (s importSet) add(spec string) {
	path := spec
	alias := ""
	if fields := strings.Fields(spec); len(fields) == 2 {
		alias, path = fields[0], fields[1]
	}
	if _, seen := s[path]; !seen || alias == "" {
		s[path] = spec
	}
}

// specs returns the kept import specs in path order.
func (s importSet) specs() []string {
	paths := make([]string, 0, len(s))
	for path := range s {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	specs := make([]string, 0, len(paths))
	for _, path := range paths {
		specs = append(specs, s[path])
	}
	return specs
}
