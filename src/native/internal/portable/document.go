package portable

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// Version is the config.json format this build reads and writes. Files of another version are never written.
const Version = 1

// ErrUnsupportedVersion means config.json was written by a build with another format version.
var ErrUnsupportedVersion = errors.New("config.json has an unsupported version")

// Document is config.json. It keeps every key it read, including keys this build does not use, so a write changes
// only the keys it sets or removes. The accessors take a path of keys, such as "settings", "paletteHotkey".
type Document struct {
	root *object
}

func newDocument() Document {
	root := &object{}
	root.set("version", json.Number(strconv.Itoa(Version)))
	root.set("settings", &object{})
	root.set("shortcuts", &object{})
	return Document{root: root}
}

func validateDocument(obj *object) error {
	version, _ := obj.get("version")
	if !isVersion(version) {
		return fmt.Errorf("%w: %v", ErrUnsupportedVersion, version)
	}
	for _, section := range []string{"settings", "shortcuts"} {
		if value, ok := obj.get(section); ok && !isObject(value) {
			return fmt.Errorf("config.json %q must be an object", section)
		}
	}
	return validateShortcutValues(obj)
}

func validateShortcutValues(obj *object) error {
	shortcuts, ok := obj.get("shortcuts")
	if !ok {
		return nil
	}
	for _, m := range shortcuts.(*object).members {
		if _, isString := m.value.(string); !isString && m.value != nil {
			return fmt.Errorf("config.json shortcuts.%s must be an accelerator string or null", m.key)
		}
	}
	return nil
}

func isObject(value any) bool {
	_, ok := value.(*object)
	return ok
}

func isVersion(value any) bool {
	number, ok := value.(json.Number)
	return ok && string(number) == "1"
}

// Value returns the raw value at path, and whether it exists.
func (d Document) Value(path ...string) (any, bool) {
	obj := d.root
	for i, key := range path {
		value, ok := obj.get(key)
		if !ok {
			return nil, false
		}
		if i == len(path)-1 {
			return value, true
		}
		next, isObject := value.(*object)
		if !isObject {
			return nil, false
		}
		obj = next
	}
	return nil, false
}

// String returns the string at path. A value of another type reads as absent.
func (d Document) String(path ...string) (string, bool) {
	value, ok := d.Value(path...)
	text, isString := value.(string)
	return text, ok && isString
}

// Bool returns the boolean at path. A value of another type reads as absent.
func (d Document) Bool(path ...string) (bool, bool) {
	value, ok := d.Value(path...)
	flag, isBool := value.(bool)
	return flag, ok && isBool
}

// Keys lists the keys of the object at path, in file order. A missing or non-object path has none.
func (d Document) Keys(path ...string) []string {
	obj := d.objectAt(path...)
	if obj == nil {
		return nil
	}
	return obj.keys()
}

func (d Document) objectAt(path ...string) *object {
	obj := d.root
	for _, key := range path {
		next, ok := obj.get(key)
		if !ok {
			return nil
		}
		child, isObject := next.(*object)
		if !isObject {
			return nil
		}
		obj = child
	}
	return obj
}

// SetValue stores value at path, creating the objects on the way and replacing a value of another kind.
func (d Document) SetValue(value any, path ...string) {
	parent := d.root
	for _, key := range path[:len(path)-1] {
		parent = parent.child(key)
	}
	parent.set(path[len(path)-1], value)
}

// Remove deletes the key at path. A missing key is not an error.
func (d Document) Remove(path ...string) {
	parent := d.objectAt(path[:len(path)-1]...)
	if parent != nil {
		parent.remove(path[len(path)-1])
	}
}

func (d Document) encode() ([]byte, error) {
	return encodeDocument(d.root)
}
