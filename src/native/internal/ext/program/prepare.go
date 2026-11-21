package program

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// MaxSourceBytes bounds a program body. It is enough for a dozen lines of api calls and keeps a model from sending a
// document through the tool.
const MaxSourceBytes = 8 << 10

// EntryPoint is the function the child calls. Its signature is func(ctx api.Context) (any, error).
const EntryPoint = "Main"

// Rejection is a program the host refused before it ran. Its message is written for the model that wrote the program.
type Rejection struct {
	Message string
}

func (r *Rejection) Error() string {
	return r.Message
}

func reject(format string, args ...any) *Rejection {
	return &Rejection{Message: fmt.Sprintf(format, args...)}
}

// Prepared is a program ready to stage: the whole source file the interpreter loads.
type Prepared struct {
	Source  string
	Imports []string
}

// Prepare checks a program body and wraps it into a source file. Line numbers in errors the interpreter reports are
// the body's own, because the generated header takes the first line of the file together with the function opening.
func Prepare(body string) (Prepared, error) {
	if len(body) > MaxSourceBytes {
		return Prepared{}, reject("the program is %d bytes; keep it under %d", len(body), MaxSourceBytes)
	}
	if strings.TrimSpace(body) == "" {
		return Prepared{}, reject("the program is empty")
	}
	if err := refuseFileStructure(body); err != nil {
		return Prepared{}, err
	}
	file, err := parseWrapped(header(nil), body)
	if err != nil {
		return Prepared{}, err
	}
	fn, err := mainFunction(file)
	if err != nil {
		return Prepared{}, err
	}
	if err := refuseConstructs(fn.Body); err != nil {
		return Prepared{}, err
	}
	imports, err := importsOf(fn.Body)
	if err != nil {
		return Prepared{}, err
	}
	prepared := Prepared{Source: header(imports) + body + "\n}\n", Imports: imports}
	return prepared, checkFinalSource(prepared)
}

// checkFinalSource parses the file the child will load, not the one the checks ran on, and checks it again: the host
// never trusts that the two headers parse the body the same way.
func checkFinalSource(prepared Prepared) error {
	file, err := parser.ParseFile(token.NewFileSet(), "program.go", prepared.Source, parser.AllErrors)
	if err != nil {
		return reject("the program does not parse: %v", err)
	}
	fn, err := mainFunction(file)
	if err != nil {
		return err
	}
	if err := refuseConstructs(fn.Body); err != nil {
		return err
	}
	allowed := allowedByName()
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if _, standard := allowed[PackageName(path)]; path != APIImportPath && !(standard && allowed[PackageName(path)] == path) {
			return reject("the program imports %s, which programs may not use", path)
		}
	}
	return nil
}

// header is everything before the body, on one line.
func header(imports []string) string {
	var b strings.Builder
	b.WriteString("package main; ")
	if len(imports) > 0 {
		b.WriteString("import (")
		for _, importPath := range imports {
			fmt.Fprintf(&b, "%q; ", importPath)
		}
		b.WriteString("); ")
	}
	fmt.Fprintf(&b, "import api %q; func %s(ctx api.Context) (any, error) {\n", APIImportPath, EntryPoint)
	return b.String()
}

// refuseFileStructure turns the mistake of sending a whole file into a message that says what to send instead.
func refuseFileStructure(body string) error {
	trimmed := strings.TrimSpace(body)
	for _, prefix := range []string{"package ", "import ", "import(", "func Main"} {
		if strings.HasPrefix(trimmed, prefix) {
			return reject("send only the statements of the function body: no package clause, imports or func Main. The host adds them; use ctx for the api and fmt, strings and the other allowed packages by name")
		}
	}
	return nil
}

// parseWrapped parses the program in its wrapper and reports a syntax error with the body's line numbers.
func parseWrapped(head, body string) (*ast.File, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "program.go", head+body+"\n}\n", parser.AllErrors)
	if err == nil {
		return file, nil
	}
	var list scanner.ErrorList
	if asList, ok := err.(scanner.ErrorList); ok {
		list = asList
	}
	if len(list) == 0 {
		return nil, reject("the program does not parse: %v", err)
	}
	lines := make([]string, 0, 3)
	for _, entry := range list {
		if len(lines) == 3 {
			break
		}
		lines = append(lines, fmt.Sprintf("line %d: %s", max(entry.Pos.Line-1, 1), entry.Msg))
	}
	return nil, reject("the program does not parse: %s", strings.Join(lines, "; "))
}

// mainFunction checks that the wrapped file holds exactly the imports and Main, so a body cannot close the function
// early and declare anything beside it.
func mainFunction(file *ast.File) (*ast.FuncDecl, error) {
	var found *ast.FuncDecl
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok != token.IMPORT {
				return nil, reject("the program declares something outside the function body; write statements only")
			}
		case *ast.FuncDecl:
			if found != nil || d.Name.Name != EntryPoint {
				return nil, reject("the program declares a function outside the function body; use a func literal inside it")
			}
			found = d
		}
	}
	if found == nil || found.Body == nil {
		return nil, reject("the program has no function body")
	}
	return found, nil
}

// refuseConstructs rejects what the interpreter cannot run safely or a program has no use for: goroutines and channels
// (a blocked receive leaks a goroutine, and the child cannot interrupt it) and goto.
func refuseConstructs(body *ast.BlockStmt) error {
	var refusal error
	ast.Inspect(body, func(node ast.Node) bool {
		if refusal != nil {
			return false
		}
		switch n := node.(type) {
		case *ast.GoStmt:
			refusal = reject("goroutines (go) are not available; run the steps one after another")
		case *ast.SelectStmt:
			refusal = reject("select is not available; programs have no channels")
		case *ast.ChanType:
			refusal = reject("channels are not available; use slices and maps")
		case *ast.SendStmt:
			refusal = reject("channels are not available; use slices and maps")
		case *ast.UnaryExpr:
			if n.Op == token.ARROW {
				refusal = reject("channels are not available; use slices and maps")
			}
		case *ast.BranchStmt:
			if n.Tok == token.GOTO {
				refusal = reject("goto is not available; use loops and returns")
			}
		}
		return refusal == nil
	})
	return refusal
}

// importsOf collects the packages the body names as qualifiers (strings.Fields, json.Marshal) and returns their import
// paths, sorted. A qualifier that is neither declared in the body, nor ctx, nor an allowed package, nor api is refused
// by name, so os.ReadFile and http.Get fail before anything runs.
func importsOf(body *ast.BlockStmt) ([]string, error) {
	allowed := allowedByName()
	used := map[string]bool{}
	var refusal error
	ast.Inspect(body, func(node ast.Node) bool {
		if refusal != nil {
			return false
		}
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		base, ok := selector.X.(*ast.Ident)
		if !ok || base.Obj != nil || base.Name == "api" {
			return true
		}
		if _, ok := allowed[base.Name]; ok {
			used[base.Name] = true
			return true
		}
		refusal = reject("%s.%s: %s is not available to programs. Use ctx for the Nevermind api, and only these packages by name: %s",
			base.Name, selector.Sel.Name, base.Name, availablePackages())
		return false
	})
	if refusal != nil {
		return nil, refusal
	}
	imports := make([]string, 0, len(used))
	for name := range used {
		imports = append(imports, allowed[name])
	}
	sort.Strings(imports)
	return imports, nil
}
