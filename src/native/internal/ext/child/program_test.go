package child

import (
	"strings"
	"testing"
)

// The program table is the second line behind the host's checks: even a source that names a package the host would
// refuse cannot load it, because the interpreter has neither its symbols nor a source tree to find it in.
func TestProgramInterpreterHasNoOSNetworkProcessOrUnsafe(t *testing.T) {
	for _, importPath := range []string{"os", "os/exec", "net", "net/http", "syscall", "unsafe", "reflect", "runtime", "io/ioutil", "path/filepath", "sync", "context"} {
		source := "package main\nimport _ \"" + importPath + "\"\nimport api \"github.com/pablopunk/nvm/native/internal/ext/api\"\nfunc Main(ctx api.Context) (any, error) { return nil, nil }\n"
		if _, err := evalProgram(source); err == nil {
			t.Errorf("import %q loaded in the program interpreter", importPath)
		}
	}
}

func TestProgramInterpreterRefusesTheBackgroundExportsOfTime(t *testing.T) {
	for _, call := range []string{"time.AfterFunc(0, func() {})", "time.NewTimer(0)", "time.After(0)", "time.Tick(1)", "time.NewTicker(1)"} {
		source := "package main\nimport \"time\"\nimport api \"github.com/pablopunk/nvm/native/internal/ext/api\"\nfunc Main(ctx api.Context) (any, error) { _ = " + call + "; return nil, nil }\n"
		if _, err := evalProgram(source); err == nil {
			t.Errorf("%s compiled", call)
		}
	}
}

func TestProgramInterpreterRunsAllowedPackages(t *testing.T) {
	source := "package main\nimport \"strings\"\nimport api \"github.com/pablopunk/nvm/native/internal/ext/api\"\nfunc Main(ctx api.Context) (any, error) { return strings.ToUpper(\"ok\"), nil }\n"
	i, err := evalProgram(source)
	if err != nil {
		t.Fatal(err)
	}
	main, err := i.Eval("Main")
	if err != nil || main.Type() != programSignature {
		t.Fatalf("Main = %v, %v", main, err)
	}
	if !strings.Contains(main.Type().String(), "api.Context") {
		t.Fatalf("type = %s", main.Type())
	}
}
