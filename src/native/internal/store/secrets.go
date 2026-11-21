package store

import (
	"fmt"
	"reflect"
)

// secretTag marks a field that must never reach disk: write it as `store:"secret"`. Tokens and extension
// secrets belong in the OS keychain (the Keychain port); this tag is the backstop for mistakes.
const secretTag = "secret"

// secretFieldIndexes returns the positions of the store:"secret" fields of a struct document. Secrets are
// only supported as top-level fields, so a secret nested deeper panics: the store could not then omit it
// from a copy without rebuilding the value.
func secretFieldIndexes[T any]() []int {
	documentType := reflect.TypeFor[T]()
	if documentType.Kind() != reflect.Struct {
		if typeHoldsSecret(documentType, map[reflect.Type]bool{}) {
			panic("store: secret fields are supported only as top-level fields of a struct document")
		}
		return nil
	}
	var indexes []int
	for i := range documentType.NumField() {
		field := documentType.Field(i)
		if field.Tag.Get("store") == secretTag {
			if !field.IsExported() {
				panic(fmt.Sprintf("store: secret field %s must be exported to be omitted", field.Name))
			}
			indexes = append(indexes, i)
			continue
		}
		if typeHoldsSecret(field.Type, map[reflect.Type]bool{}) {
			panic(fmt.Sprintf("store: field %s nests a secret; secret fields must be top-level", field.Name))
		}
	}
	return indexes
}

// typeHoldsSecret reports whether a secret tag appears anywhere inside t. seen stops recursion through
// self-referencing types.
func typeHoldsSecret(t reflect.Type, seen map[reflect.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return typeHoldsSecret(t.Elem(), seen)
	case reflect.Map:
		return typeHoldsSecret(t.Key(), seen) || typeHoldsSecret(t.Elem(), seen)
	case reflect.Struct:
		for i := range t.NumField() {
			field := t.Field(i)
			if field.Tag.Get("store") == secretTag || typeHoldsSecret(field.Type, seen) {
				return true
			}
		}
	}
	return false
}

// redactSecrets returns a shallow copy of value with the secret fields zeroed, and the names of the fields
// that held a value. The original value is not changed.
func redactSecrets[T any](value T, secrets []int) (T, []string) {
	if len(secrets) == 0 {
		return value, nil
	}
	redacted := value
	fields := reflect.ValueOf(&redacted).Elem()
	var omitted []string
	for _, index := range secrets {
		field := fields.Field(index)
		if field.IsZero() {
			continue
		}
		field.Set(reflect.Zero(field.Type()))
		omitted = append(omitted, fields.Type().Field(index).Name)
	}
	return redacted, omitted
}
