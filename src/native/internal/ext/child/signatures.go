package child

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// entryPointSignatures is the Go type each entry point must have: the method of the api interface that the host
// calls, read from the api package itself, so a changed api changes the expected types here.
var entryPointSignatures = map[string]reflect.Type{
	protocol.MethodCommands:        interfaceMethodType((*api.CommandProvider)(nil), "Commands"),
	protocol.MethodRoot:            interfaceMethodType((*api.RootProvider)(nil), "RootItems"),
	protocol.MethodSearch:          interfaceMethodType((*api.SearchProvider)(nil), "SearchItems"),
	protocol.MethodRun:             interfaceMethodType((*api.ActionHandler)(nil), "RunAction"),
	protocol.MethodLoad:            interfaceMethodType((*api.DataLoader)(nil), "LoadItems"),
	protocol.MethodMenuBars:        interfaceMethodType((*api.MenuBarProvider)(nil), "MenuBars"),
	protocol.MethodMenuBarTriggers: interfaceMethodType((*api.MenuBarProvider)(nil), "MenuBarTriggers"),
}

// interfaceMethodType returns the function type of method name in the api interface that pointer points to. It
// panics when the api has no such method: the table above is then out of date with the api package.
func interfaceMethodType(pointer any, name string) reflect.Type {
	method, ok := reflect.TypeOf(pointer).Elem().MethodByName(name)
	if !ok {
		panic(fmt.Sprintf("the api has no method %s on %s", name, reflect.TypeOf(pointer).Elem()))
	}
	return method.Type
}

// checkEntryPointSignatures refuses the first entry point, in method order, whose type is not the api's signature.
// A source whose function has another type would fail at its first call, so it is refused at load, named by function.
func checkEntryPointSignatures(functions map[string]reflect.Value) error {
	methods := make([]string, 0, len(functions))
	for method := range functions {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	for _, method := range methods {
		got := functions[method].Type()
		want := entryPointSignatures[method]
		if got != want {
			return fmt.Errorf("%s has the wrong signature: it is %s, want %s (see the API reference)",
				entryPoints[method], got, want)
		}
	}
	return nil
}
