package api

import (
	"fmt"
	"reflect"
	"testing"
)

// minimumWireTypes guards against the walk silently visiting nothing.
const minimumWireTypes = 100

// TestSerializableTypesAreCloneSafe walks the type graph of every wire type. A func, chan,
// interface or unsafe pointer anywhere reachable from a wire type would make the payload unsafe to
// clone across Electron IPC or the subprocess boundary, so the walk fails the build instead.
// It also requires an explicit json tag on every exported non-embedded field, which keeps the
// wire names stable when Go identifiers are refactored.
func TestSerializableTypesAreCloneSafe(t *testing.T) {
	violations, visited := cloneViolations(SerializableTypes())
	for _, violation := range violations {
		t.Error(violation)
	}
	if visited < minimumWireTypes {
		t.Fatalf("walk visited %d types; the wire type graph is smaller than expected", visited)
	}
}

// TestCloneWalkFlagsForbiddenShapes proves the walk can fail: each local type breaks one rule.
func TestCloneWalkFlagsForbiddenShapes(t *testing.T) {
	type withFunc struct {
		Handler func() `json:"handler"`
	}
	type withChan struct {
		Events chan int `json:"events"`
	}
	type withAny struct {
		Payload any `json:"payload"`
	}
	type nestedAny struct {
		Inner withAny `json:"inner"`
	}
	// The fields below exist only so that reflection sees them; nothing reads them.
	type untagged struct {
		//lint:ignore U1000 the field is inspected by reflection in the walk, never read
		Name string
	}
	type unexported struct {
		//lint:ignore U1000 the field is inspected by reflection in the walk, never read
		name string
	}
	tests := []struct {
		name string
		typ  reflect.Type
	}{
		{"func field", reflect.TypeFor[withFunc]()},
		{"chan field", reflect.TypeFor[withChan]()},
		{"interface field", reflect.TypeFor[withAny]()},
		{"interface reached through a nested struct", reflect.TypeFor[nestedAny]()},
		{"missing json tag", reflect.TypeFor[untagged]()},
		{"unexported field", reflect.TypeFor[unexported]()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations, _ := cloneViolations([]reflect.Type{tt.typ})
			if len(violations) == 0 {
				t.Fatalf("%s was not flagged", tt.name)
			}
		})
	}
}

// cloneViolations returns one message per forbidden shape and the number of distinct types visited.
func cloneViolations(roots []reflect.Type) ([]string, int) {
	walker := &typeWalker{seen: map[reflect.Type]bool{}}
	for _, root := range roots {
		walker.walk(root.Name(), root)
	}
	return walker.violations, len(walker.seen)
}

type typeWalker struct {
	seen       map[reflect.Type]bool
	violations []string
}

func (w *typeWalker) report(format string, args ...any) {
	w.violations = append(w.violations, fmt.Sprintf(format, args...))
}

func (w *typeWalker) walk(path string, typ reflect.Type) {
	if w.seen[typ] {
		return
	}
	w.seen[typ] = true
	switch typ.Kind() {
	case reflect.Func, reflect.Chan, reflect.Interface, reflect.UnsafePointer:
		w.report("%s is %s; wire types must be plain data", path, typ.Kind())
	case reflect.Pointer, reflect.Slice, reflect.Array:
		w.walk(path, typ.Elem())
	case reflect.Map:
		w.walk(path+"[key]", typ.Key())
		w.walk(path+"[value]", typ.Elem())
	case reflect.Struct:
		w.walkStruct(path, typ)
	}
}

func (w *typeWalker) walkStruct(path string, typ reflect.Type) {
	for i := range typ.NumField() {
		field := typ.Field(i)
		fieldPath := path + "." + field.Name
		if field.Tag.Get("json") == "-" {
			continue
		}
		if !field.IsExported() {
			w.report("%s is unexported; it would be silently dropped on the wire", fieldPath)
			continue
		}
		if !field.Anonymous && field.Tag.Get("json") == "" {
			w.report("%s has no json tag; wire names must be explicit", fieldPath)
		}
		w.walk(fieldPath, field.Type)
	}
}

func TestCloneWalkSkipsHostOnlyFields(t *testing.T) {
	type hostOnly struct {
		Callback func() `json:"-"`
		Public   string `json:"public"`
	}
	violations, _ := cloneViolations([]reflect.Type{reflect.TypeFor[hostOnly]()})
	if len(violations) != 0 {
		t.Fatalf("host-only field reached wire walk: %v", violations)
	}
}
