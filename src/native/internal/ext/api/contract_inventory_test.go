package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type contractEntry struct {
	Interface      string   `json:"interface"`
	Kind           string   `json:"kind"`
	Package        string   `json:"package"`
	Implementation string   `json:"implementation"`
	Methods        []string `json:"methods"`
}

func TestAPIContractInventoryCoversEveryInterfaceAndConcreteMethod(t *testing.T) {
	var entries []contractEntry
	readParityJSON(t, "api-contract.json", &entries)
	interfaces := map[string][]string{}
	for _, file := range apiSources(t) {
		for _, decl := range file.Decls {
			group, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range group.Specs {
				named, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				contract, ok := named.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				for _, field := range contract.Methods.List {
					for _, name := range field.Names {
						interfaces[named.Name.Name] = append(interfaces[named.Name.Name], name.Name)
					}
				}
				slices.Sort(interfaces[named.Name.Name])
			}
		}
	}
	directories := map[string]string{"child": "../child", "api": ".", "platform/real": "../../platform/real"}
	seen := map[string]bool{}
	for _, entry := range entries {
		if seen[entry.Interface] {
			t.Errorf("duplicate contract for %s", entry.Interface)
		}
		seen[entry.Interface] = true
		methods, ok := interfaces[entry.Interface]
		if !ok || !slices.Equal(methods, entry.Methods) {
			t.Errorf("%s changed; classify its current members in api-contract.json", entry.Interface)
		}
		switch entry.Kind {
		case "extension-callback":
			if entry.Implementation != "" || entry.Package != "" {
				t.Errorf("callback %s cannot claim a host implementation", entry.Interface)
			}
			continue
		case "service", "local", "context":
		default:
			t.Errorf("%s has unknown contract kind %q", entry.Interface, entry.Kind)
			continue
		}
		directory, ok := directories[entry.Package]
		if !ok || entry.Implementation == "" {
			t.Errorf("%s needs a concrete implementation", entry.Interface)
			continue
		}
		concrete := concreteContractMethods(t, directory, entry.Implementation)
		for _, method := range entry.Methods {
			if !concrete[method] {
				t.Errorf("%s.%s is not concretely implemented by %s; embedded nil interfaces do not count", entry.Interface, method, entry.Implementation)
			}
		}
	}
	for name := range interfaces {
		if !seen[name] {
			t.Errorf("classify new API interface %s in api-contract.json", name)
		}
	}
}

func concreteContractMethods(t *testing.T, directory, implementation string) map[string]bool {
	t.Helper()
	files, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]bool{}
	for _, entry := range files {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "api_symbols.go" {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			receiver := fn.Recv.List[0].Type
			if pointer, ok := receiver.(*ast.StarExpr); ok {
				receiver = pointer.X
			}
			if named, ok := receiver.(*ast.Ident); ok && named.Name == implementation {
				methods[fn.Name.Name] = true
			}
		}
	}
	return methods
}
