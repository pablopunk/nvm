package api

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// maxOpenLedgerRows ratchets the 1.x members with no decision yet. Lower it as rows close; it never goes up.
const maxOpenLedgerRows = 13

const parityDir = "../../../testdata/parity"

type ledgerRow struct {
	Member string `json:"member"`
	Status string `json:"status"`
	Target string `json:"target"`
	Note   string `json:"note"`
}

type apiIndex struct {
	types     map[string]map[string]bool
	functions map[string]bool
	constants map[string]bool
	corpus    string
}

func readParityJSON(t *testing.T, name string, into any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(parityDir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func fieldNames(fields *ast.FieldList, into map[string]bool) {
	for _, field := range fields.List {
		for _, name := range field.Names {
			into[name.Name] = true
		}
	}
}

func indexDeclaration(decl ast.Decl, index *apiIndex) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Recv == nil {
			index.functions[d.Name.Name] = true
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				members := map[string]bool{}
				switch typ := s.Type.(type) {
				case *ast.StructType:
					fieldNames(typ.Fields, members)
				case *ast.InterfaceType:
					fieldNames(typ.Methods, members)
				}
				index.types[s.Name.Name] = members
			case *ast.ValueSpec:
				for _, name := range s.Names {
					index.constants[name.Name] = true
				}
			}
		}
	}
}

func buildAPIIndex(t *testing.T) *apiIndex {
	t.Helper()
	index := &apiIndex{types: map[string]map[string]bool{}, functions: map[string]bool{}, constants: map[string]bool{}}
	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range packages["api"].Files {
		for _, decl := range file.Decls {
			indexDeclaration(decl, index)
		}
	}
	sources, _ := filepath.Glob("*.go")
	sources = append(sources, "../../windowmgmt/failure.go", "../../platform/capabilities.go")
	var corpus strings.Builder
	for _, source := range sources {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		corpus.Write(data)
	}
	index.corpus = corpus.String()
	return index
}

func (index *apiIndex) resolves(target string) bool {
	kind, name, qualified := strings.Cut(target, ":")
	if !qualified {
		name, kind = target, ""
	}
	switch kind {
	case "value":
		return regexp.MustCompile(`(^|[^\w-])` + regexp.QuoteMeta(name) + `($|[^\w-])`).MatchString(index.corpus)
	case "func":
		return index.functions[name]
	case "const":
		return index.constants[name]
	}
	typeName, member, hasMember := strings.Cut(name, ".")
	members, known := index.types[typeName]
	return known && (!hasMember || members[member])
}

// TestLedgerCoversEveryMemberOfThe1xAPI is gate G1 of docs/extension-api-parity-plan.md: every member of the 1.x API
// has a decision, and every decision names something that exists.
func TestLedgerCoversEveryMemberOfThe1xAPI(t *testing.T) {
	var members []string
	var rows []ledgerRow
	readParityJSON(t, "api-1x-members.json", &members)
	readParityJSON(t, "api-ledger.json", &rows)
	byMember := make(map[string]ledgerRow, len(rows))
	for _, row := range rows {
		byMember[row.Member] = row
	}
	index := buildAPIIndex(t)
	open := 0
	for _, member := range members {
		row, found := byMember[member]
		if !found {
			t.Errorf("%s has no ledger row", member)
			continue
		}
		switch row.Status {
		case "open":
			open++
		case "dropped":
			if row.Note == "" {
				t.Errorf("%s is dropped with no reason", member)
			}
		case "same", "renamed", "better":
			if !index.resolves(row.Target) {
				t.Errorf("%s points at %q, which does not exist", member, row.Target)
			}
		default:
			t.Errorf("%s has unknown status %q", member, row.Status)
		}
	}
	if open > maxOpenLedgerRows {
		t.Errorf("%d rows are open; the limit is %d", open, maxOpenLedgerRows)
	}
}
