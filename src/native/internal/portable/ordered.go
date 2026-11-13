package portable

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// member is one key of an object, kept with the position the file gave it.
type member struct {
	key   string
	value any
}

// object is a JSON object that keeps its key order, so a rewrite changes only the keys it means to change, and the
// diff a dotfiles repository shows stays small. Values are *object, []any, string, bool, nil or json.Number.
type object struct {
	members []member
}

func (o *object) index(key string) int {
	for i, m := range o.members {
		if m.key == key {
			return i
		}
	}
	return -1
}

func (o *object) get(key string) (any, bool) {
	if i := o.index(key); i >= 0 {
		return o.members[i].value, true
	}
	return nil, false
}

func (o *object) set(key string, value any) {
	if i := o.index(key); i >= 0 {
		o.members[i].value = value
		return
	}
	o.members = append(o.members, member{key: key, value: value})
}

func (o *object) remove(key string) {
	if i := o.index(key); i >= 0 {
		o.members = append(o.members[:i], o.members[i+1:]...)
	}
}

// child returns the object under key, replacing any other value there with an object first.
func (o *object) child(key string) *object {
	if existing, ok := o.get(key); ok {
		if obj, isObject := existing.(*object); isObject {
			return obj
		}
	}
	obj := &object{}
	o.set(key, obj)
	return obj
}

func (o *object) keys() []string {
	keys := make([]string, 0, len(o.members))
	for _, m := range o.members {
		keys = append(keys, m.key)
	}
	return keys
}

var errNotJSON = errors.New("config is not a JSON document")

func decodeDocument(data []byte) (*object, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	value, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	obj, ok := value.(*object)
	if !ok {
		return nil, errNotJSON
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errNotJSON
	}
	return obj, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return token, nil
	}
	switch delim {
	case '{':
		return decodeObject(dec)
	case '[':
		return decodeArray(dec)
	}
	return nil, fmt.Errorf("unexpected %v", delim)
}

func decodeObject(dec *json.Decoder) (*object, error) {
	obj := &object{}
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, errNotJSON
		}
		value, err := decodeValue(dec)
		if err != nil {
			return nil, err
		}
		obj.set(key, value)
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return obj, nil
}

func decodeArray(dec *json.Decoder) ([]any, error) {
	items := []any{}
	for dec.More() {
		value, err := decodeValue(dec)
		if err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return items, nil
}

// encodeDocument writes obj the way 1.x writes config.json: JSON.stringify with two-space indentation, a trailing
// newline, and no HTML escaping.
func encodeDocument(obj *object) ([]byte, error) {
	var compact bytes.Buffer
	if err := writeCompact(&compact, obj); err != nil {
		return nil, err
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, compact.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	indented.WriteByte('\n')
	return indented.Bytes(), nil
}

func writeCompact(buf *bytes.Buffer, value any) error {
	switch v := value.(type) {
	case *object:
		return writeObject(buf, v)
	case []any:
		return writeArray(buf, v)
	case nil:
		buf.WriteString("null")
	case json.Number:
		buf.WriteString(v.String())
	case bool:
		buf.WriteString(strconv.FormatBool(v))
	case string:
		return writeString(buf, v)
	default:
		return fmt.Errorf("config holds an unsupported value %T", value)
	}
	return nil
}

func writeObject(buf *bytes.Buffer, obj *object) error {
	buf.WriteByte('{')
	for i, m := range obj.members {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := writeString(buf, m.key); err != nil {
			return err
		}
		buf.WriteByte(':')
		if err := writeCompact(buf, m.value); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

func writeArray(buf *bytes.Buffer, items []any) error {
	buf.WriteByte('[')
	for i, item := range items {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := writeCompact(buf, item); err != nil {
			return err
		}
	}
	buf.WriteByte(']')
	return nil
}

func writeString(buf *bytes.Buffer, s string) error {
	var encoded bytes.Buffer
	enc := json.NewEncoder(&encoded)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return err
	}
	buf.Write(bytes.TrimSuffix(encoded.Bytes(), []byte("\n")))
	return nil
}
