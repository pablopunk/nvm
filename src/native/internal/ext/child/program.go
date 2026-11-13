package child

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"reflect"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/program"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// mode is what a child serves: one extension, or one program of an AI conversation.
type mode int

const (
	modeExtension mode = iota
	modeProgram
)

// programSignature is the type of Main: the program body wrapped into func(ctx api.Context) (any, error).
var programSignature = reflect.TypeOf((func(api.Context) (any, error))(nil))

// loadIn loads the source at path for the child's mode.
func loadIn(m mode, path string, identity api.Identity, clock platform.Clock, link *hostLink) (*extension, error) {
	if m == modeProgram {
		return loadProgram(path, identity, clock, link)
	}
	return load(path, identity, clock, link)
}

// loadProgram interprets a staged program with the restricted symbol table. The host wrote the file from a checked
// body (package program), and the table is the second line: a package outside the allowlist is simply not there.
func loadProgram(path string, identity api.Identity, clock platform.Clock, link *hostLink) (*extension, error) {
	source, err := loadSource(path)
	if err != nil {
		return nil, err
	}
	i, err := evalProgram(source)
	if err != nil {
		return nil, err
	}
	main, err := i.Eval(program.EntryPoint)
	if err != nil || !main.IsValid() || main.Type() != programSignature {
		return nil, errors.New("the program has no Main with the signature func(ctx api.Context) (any, error)")
	}
	return &extension{functions: map[string]reflect.Value{protocol.MethodProgram: main}, base: newHostContext(identity, clock, link), program: true}, nil
}

// evalProgram runs the source in an interpreter that knows only the allowed packages and the api. It has no file system
// to load a package from source, which is how the interpreter would otherwise find one the table lacks.
func evalProgram(source string) (i *interp.Interpreter, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("the program panicked while loading: %v", recovered)
		}
	}()
	i = interp.New(interp.Options{GoPath: "-", SourcecodeFilesystem: emptySources()})
	if err := i.Use(program.Symbols(stdlib.Symbols, Symbols)); err != nil {
		return nil, err
	}
	if _, err := i.Eval(source); err != nil {
		return nil, fmt.Errorf("the program does not compile: %w", err)
	}
	return i, nil
}

// noSources is a file system with no files.
type noSources struct{}

func (noSources) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func emptySources() fs.FS {
	return noSources{}
}

// runProgram calls Main and returns its value as JSON, within MaxProgramResultBytes.
func (e *extension) runProgram() (json.RawMessage, error) {
	out, err := e.invoke(protocol.MethodProgram, e.contextArgument(api.LaunchContext{}))
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(out[0].Interface())
	if err != nil {
		return nil, fmt.Errorf("the program returned a value that cannot be written as JSON (%v): %w", err, api.ErrInvalidInput)
	}
	if len(data) > protocol.MaxProgramResultBytes {
		return nil, fmt.Errorf("the program returned %d bytes; return less than %d, such as counts, ids and short excerpts: %w", len(data), protocol.MaxProgramResultBytes, api.ErrQuotaExceeded)
	}
	return data, nil
}
