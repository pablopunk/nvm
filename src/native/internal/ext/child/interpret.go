package child

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// entryPoints names the top-level function each protocol method calls in extension source. The interpreter does
// not expose methods of interpreted struct values through reflection, so entry points are functions looked up
// by name at load time. A missing entry point means the extension does not provide that capability.
var entryPoints = map[string]string{
	protocol.MethodCommands: "Commands",
	protocol.MethodRoot:     "RootItems",
	protocol.MethodSearch:   "SearchItems",
	protocol.MethodRun:      "RunAction",
}

// extension is one interpreted extension: the functions it declares and the identity the host gave it.
type extension struct {
	functions map[string]reflect.Value
	identity  api.Identity
}

// load interprets the extension source at path. Extension code runs here, at load, inside the child; the host
// has already reviewed extension.json.
func load(path string, identity api.Identity) (*extension, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read extension source: %w", err)
	}
	i, err := evalExtension(string(source))
	if err != nil {
		return nil, err
	}
	return &extension{functions: lookupEntryPoints(i), identity: identity}, nil
}

// evalExtension runs the source in a fresh interpreter. A panic raised while the interpreter loads the source,
// such as one from package initialisation, becomes an error.
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

// callContext is the first argument of every entry point. It is a plain map, so extension source needs no import
// of the host's packages: today it carries the identity and the launch reason.
func (e *extension) callContext(launch api.LaunchContext) reflect.Value {
	return reflect.ValueOf(map[string]any{
		"id":     e.identity.ID,
		"title":  e.identity.Title,
		"reason": launch.Reason,
	})
}
