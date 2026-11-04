package child

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// entryPoints names the top-level function each protocol method calls in extension source. The functions have
// the signatures of the api interfaces they implement (CommandProvider, RootProvider, SearchProvider,
// ActionHandler). The interpreter does not expose methods of interpreted struct values through reflection, so
// entry points are looked up by name at load time. A missing entry point means the extension does not provide
// that capability.
var entryPoints = map[string]string{
	protocol.MethodCommands: "Commands",
	protocol.MethodRoot:     "RootItems",
	protocol.MethodSearch:   "SearchItems",
	protocol.MethodRun:      "RunAction",
	protocol.MethodLoad:     "LoadItems",

	protocol.MethodMenuBars:        "MenuBars",
	protocol.MethodMenuBarTriggers: "MenuBarTriggers",
}

// extension is one interpreted extension: the functions it declares and the services it may use.
type extension struct {
	functions map[string]reflect.Value
	base      hostContext
}

// load interprets the extension source at path. Extension code runs here, at load, inside the child; the host
// has already reviewed extension.json.
func load(path string, identity api.Identity, clock platform.Clock, link *hostLink) (*extension, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read extension source: %w", err)
	}
	i, err := evalExtension(string(source))
	if err != nil {
		return nil, err
	}
	functions := lookupEntryPoints(i)
	if err := checkEntryPointArities(functions); err != nil {
		return nil, err
	}
	return &extension{functions: functions, base: newHostContext(identity, clock, link)}, nil
}

// entryPointArities is the argument and result count of each entry point's signature. A source whose function has
// another count would fail at its first call, so it is refused at load with the name of the function.
var entryPointArities = map[string][2]int{
	protocol.MethodCommands:        {1, 2},
	protocol.MethodRoot:            {1, 2},
	protocol.MethodSearch:          {2, 2},
	protocol.MethodRun:             {2, 2},
	protocol.MethodLoad:            {2, 2},
	protocol.MethodMenuBars:        {1, 2},
	protocol.MethodMenuBarTriggers: {0, 1},
}

// checkEntryPointArities reports the first entry point whose parameter or result count is not the api signature.
func checkEntryPointArities(functions map[string]reflect.Value) error {
	for method, function := range functions {
		want := entryPointArities[method]
		if function.Type().NumIn() != want[0] || function.Type().NumOut() != want[1] {
			return fmt.Errorf("%s has the wrong signature: it takes %d arguments and returns %d values, want %d and %d (see the API reference)",
				entryPoints[method], function.Type().NumIn(), function.Type().NumOut(), want[0], want[1])
		}
	}
	return nil
}

// evalExtension runs the source in a fresh interpreter that knows the standard library and the api package. A
// panic raised while the interpreter loads the source, such as one from package initialisation, becomes an error.
func evalExtension(source string) (i *interp.Interpreter, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("extension source panicked while loading: %v", recovered)
		}
	}()
	i = interp.New(interp.Options{})
	if err := i.Use(stdlib.Symbols); err != nil {
		return nil, err
	}
	if err := i.Use(Symbols); err != nil {
		return nil, err
	}
	if _, err := i.Eval(source); err != nil {
		return nil, fmt.Errorf("extension source does not compile: %w", err)
	}
	return i, nil
}

// lookupEntryPoints collects every declared entry point. Undeclared names are simply absent.
func lookupEntryPoints(i *interp.Interpreter) map[string]reflect.Value {
	functions := map[string]reflect.Value{}
	for method, name := range entryPoints {
		value, err := i.Eval(name)
		if err != nil || !value.IsValid() || value.Kind() != reflect.Func {
			continue
		}
		functions[method] = value
	}
	return functions
}

// declaredMethods lists the protocol methods the loaded source implements, in a stable order.
func (e *extension) declaredMethods() []string {
	methods := make([]string, 0, len(e.functions))
	for method := range e.functions {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

// invoke calls one entry point. It returns the results without a trailing error, and that error separately.
// A missing entry point is api.ErrNotAvailable, so the host can say the extension does not provide it.
func (e *extension) invoke(name string, args ...reflect.Value) ([]reflect.Value, error) {
	function, declared := e.functions[name]
	if !declared {
		return nil, fmt.Errorf("extension does not implement %s: %w", name, api.ErrNotAvailable)
	}
	out := function.Call(args)
	if n := len(out); n > 0 && out[n-1].Kind() == reflect.Interface {
		last := out[n-1]
		out = out[:n-1]
		if !last.IsNil() {
			failure, ok := last.Interface().(error)
			if !ok {
				return out, errors.New("extension returned a non-error in its error position")
			}
			return out, failure
		}
	}
	return out, nil
}

// contextArgument is the api.Context passed as the first argument of every entry point, carrying the launch
// reason of this call.
func (e *extension) contextArgument(launch api.LaunchContext) reflect.Value {
	ctx := e.base
	ctx.launch = launch
	return reflect.ValueOf(ctx)
}
