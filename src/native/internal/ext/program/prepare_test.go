package program

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestPrepareGeneratesImportsFromTheQualifiersTheBodyUses(t *testing.T) {
	prepared, err := Prepare("title := strings.ToUpper(fmt.Sprint(1))\nreturn json.Marshal(title)")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"encoding/json", "fmt", "strings"}; !reflect.DeepEqual(prepared.Imports, want) {
		t.Fatalf("imports = %v; want %v", prepared.Imports, want)
	}
	if !strings.HasPrefix(prepared.Source, "package main; import (") || !strings.Contains(prepared.Source, "func Main(ctx api.Context) (any, error) {\n") {
		t.Fatalf("source = %q", prepared.Source)
	}
}

func TestPrepareKeepsALocalNameThatLooksLikeAPackage(t *testing.T) {
	prepared, err := Prepare("strings := []string{\"a\"}\nreturn len(strings), nil")
	if err != nil || len(prepared.Imports) != 0 {
		t.Fatalf("Prepare = %+v, %v; want no import for a local variable", prepared, err)
	}
}

// Every package that reaches outside the program is refused by name before anything is staged.
func TestPrepareRefusesPackagesThatReachOutsideTheProgram(t *testing.T) {
	for _, qualifier := range []string{"os.ReadFile(\"/etc/hostname\")", "http.Get(\"https://example.com\")", "exec.Command(\"ls\")", "syscall.Getpid()", "unsafe.Sizeof(1)", "reflect.TypeOf(1)", "runtime.GC()", "net.Dial(\"tcp\", \"x\")", "ioutil.ReadFile(\"x\")", "filepath.Walk(\"/\", nil)"} {
		body := "_ = " + qualifier + "\nreturn nil, nil"
		_, err := Prepare(body)
		var rejection *Rejection
		if !errors.As(err, &rejection) || !strings.Contains(rejection.Message, "not available to programs") {
			t.Errorf("%s: err = %v; want a refusal that names the package", qualifier, err)
		}
	}
}

func TestPrepareRefusesGoroutinesChannelsAndGoto(t *testing.T) {
	cases := map[string]string{
		"go":      "go func() {}()\nreturn nil, nil",
		"chan":    "var c chan int\n_ = c\nreturn nil, nil",
		"make":    "c := make(chan int)\n_ = c\nreturn nil, nil",
		"select":  "select {}\n",
		"receive": "var c = ctx\n_ = <-c\nreturn nil, nil",
		"goto":    "goto end\nend:\nreturn nil, nil",
	}
	for name, body := range cases {
		if _, err := Prepare(body); err == nil {
			t.Errorf("%s: the program was accepted", name)
		}
	}
}

func TestPrepareRefusesWholeFilesAndBodiesThatEscapeTheFunction(t *testing.T) {
	cases := map[string]string{
		"package": "package main\nfunc Main(ctx api.Context) (any, error) { return nil, nil }",
		"import":  "import \"os\"\nreturn nil, nil",
		"escape":  "return nil, nil\n}\nfunc init() { os.Exit(1) }\nfunc other() {",
		"var":     "return nil, nil\n}\nvar x = 1\nfunc other() {",
		"empty":   "  \n",
		"big":     strings.Repeat("x := 1\n_ = x\n", MaxSourceBytes),
	}
	for name, body := range cases {
		if _, err := Prepare(body); err == nil {
			t.Errorf("%s: the program was accepted", name)
		}
	}
}

func TestPrepareReportsSyntaxErrorsWithTheBodysLineNumbers(t *testing.T) {
	_, err := Prepare("x := 1\ny := (\nreturn nil, nil")
	if err == nil || !strings.Contains(err.Error(), "line 3") && !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("err = %v; want a line number counted from the body", err)
	}
}

func TestSymbolsKeepOnlyAllowedPackagesAndTheAPI(t *testing.T) {
	standard := map[string]map[string]reflect.Value{
		"fmt/fmt":       {"Sprint": reflect.ValueOf(1)},
		"os/os":         {"ReadFile": reflect.ValueOf(1)},
		"net/http/http": {"Get": reflect.ValueOf(1)},
		"time/time":     {"Now": reflect.ValueOf(1), "AfterFunc": reflect.ValueOf(1), "NewTimer": reflect.ValueOf(1)},
	}
	host := map[string]map[string]reflect.Value{
		APIImportPath + "/api": {"Context": reflect.ValueOf(1)},
		"other/other":          {"X": reflect.ValueOf(1)},
	}
	table := Symbols(standard, host)
	if _, ok := table["os/os"]; ok {
		t.Fatal("os is in the table")
	}
	if _, ok := table["net/http/http"]; ok {
		t.Fatal("net/http is in the table")
	}
	if _, ok := table["other/other"]; ok {
		t.Fatal("a host package other than api is in the table")
	}
	if _, ok := table[APIImportPath+"/api"]; !ok {
		t.Fatal("api is missing")
	}
	if _, ok := table["time/time"]["Now"]; !ok {
		t.Fatal("time.Now is missing")
	}
	if _, ok := table["time/time"]["AfterFunc"]; ok {
		t.Fatal("time.AfterFunc is in the table")
	}
}
