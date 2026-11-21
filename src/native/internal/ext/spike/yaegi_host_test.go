//go:build spike

package spike

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"github.com/traefik/yaegi/stdlib/unrestricted"
)

// Item mirrors the host row type that extensions see as nvm.Item.
type Item struct {
	Title    string
	Subtitle string
	Value    string
}

// tickCount is bumped by the interpreted code through nvm.Tick. It lets a test observe whether an
// interpreted loop keeps running after the host has stopped waiting for it.
var tickCount atomic.Int64

// nowNanos is exposed as nvm.NowNano so interpreted busy loops can measure wall-clock time.
func nowNanos() int64 { return time.Now().UnixNano() }

// nvmSymbols is the host package "nvm" visible to extensions.
var nvmSymbols = interp.Exports{
	"nvm/nvm": {
		"Item":    reflect.ValueOf((*Item)(nil)),
		"Tick":    reflect.ValueOf(func() { tickCount.Add(1) }),
		"NowNano": reflect.ValueOf(nowNanos),
	},
}

// yaegiExt is one loaded extension: its own interpreter and the evaluated Search function.
type yaegiExt struct {
	interp *interp.Interpreter
	search reflect.Value
}

// newInterpreter builds an interpreter with the full stdlib table and the nvm host package.
// This is the cost every extension pays today, because each interpreter copies the symbol table.
func newInterpreter() (*interp.Interpreter, error) {
	i := interp.New(interp.Options{})
	if err := i.Use(stdlib.Symbols); err != nil {
		return nil, fmt.Errorf("use stdlib: %w", err)
	}
	if err := i.Use(nvmSymbols); err != nil {
		return nil, fmt.Errorf("use nvm: %w", err)
	}
	return i, nil
}

// loadYaegi evaluates source and returns its Search function.
func loadYaegi(source string) (*yaegiExt, error) {
	i, err := newInterpreter()
	if err != nil {
		return nil, err
	}
	if _, err := i.Eval(source); err != nil {
		return nil, fmt.Errorf("eval source: %w", err)
	}
	search, err := i.Eval("Search")
	if err != nil {
		return nil, fmt.Errorf("find Search: %w", err)
	}
	return &yaegiExt{interp: i, search: search}, nil
}

// Search calls the interpreted function through reflection, which is how a host would call it.
func (e *yaegiExt) Search(query string) ([]Item, error) {
	out := e.search.Call([]reflect.Value{reflect.ValueOf(query)})
	items, ok := out[0].Interface().([]Item)
	if !ok {
		return nil, fmt.Errorf("Search returned %T", out[0].Interface())
	}
	return items, nil
}

// evalUnrestricted is evalValue with interp.Options.Unrestricted and the unrestricted symbol set,
// which is the only way Yaegi v0.16.1 exposes os/exec.
func evalUnrestricted(source, expression string) (string, error) {
	i := interp.New(interp.Options{Unrestricted: true})
	if err := i.Use(stdlib.Symbols); err != nil {
		return "", err
	}
	if err := i.Use(unrestricted.Symbols); err != nil {
		return "", err
	}
	if _, err := i.Eval(source); err != nil {
		return "", fmt.Errorf("eval source: %w", err)
	}
	value, err := i.Eval(expression)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(value.Interface()), nil
}

// evalValue evaluates a single expression such as Run() and returns its printed form.
func evalValue(source, expression string) (string, error) {
	i, err := newInterpreter()
	if err != nil {
		return "", err
	}
	if _, err := i.Eval(source); err != nil {
		return "", fmt.Errorf("eval source: %w", err)
	}
	value, err := i.Eval(expression)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(value.Interface()), nil
}
