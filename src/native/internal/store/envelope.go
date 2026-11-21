package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// storedEnvelope is a parsed file: the schema version it was written with and its raw data.
type storedEnvelope struct {
	version int
	data    json.RawMessage
}

// decodeEnvelope parses a stored file. When the file is not a well-formed envelope it returns a short
// reason and never any of the content. Unknown top-level keys are allowed so that a newer build can add
// fields without its files being mistaken for corrupt ones.
func decodeEnvelope(raw []byte) (storedEnvelope, string) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return storedEnvelope{}, "file is empty"
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var fields struct {
		SchemaVersion *int            `json:"schemaVersion"`
		Data          json.RawMessage `json:"data"`
	}
	if err := decoder.Decode(&fields); err != nil {
		return storedEnvelope{}, damageReason(err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return storedEnvelope{}, "trailing content after the document"
	}
	if fields.SchemaVersion == nil || *fields.SchemaVersion < 1 || isAbsentJSON(fields.Data) {
		return storedEnvelope{}, "not an envelope with schemaVersion and data"
	}
	return storedEnvelope{version: *fields.SchemaVersion, data: fields.Data}, ""
}

// isAbsentJSON reports whether data is missing or the JSON literal null.
func isAbsentJSON(data json.RawMessage) bool {
	return len(data) == 0 || string(bytes.TrimSpace(data)) == "null"
}

// damageReason turns a decoding error into a reason that carries no content.
func damageReason(err error) string {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return fmt.Sprintf("not valid JSON at byte %d", syntax.Offset)
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return "not valid JSON: the file ends early"
	}
	return "not an envelope with schemaVersion and data"
}

// typedEnvelope is the on-disk shape of every document, {"schemaVersion": N, "data": ...}. It encodes the
// document value directly, in one pass.
type typedEnvelope[T any] struct {
	SchemaVersion int `json:"schemaVersion"`
	Data          T   `json:"data"`
}

// encodeLocked renders the document as an indented envelope with secret fields omitted. It returns the
// omitted field names, which carry no values. The caller holds c.mu.
func (c *Collection[T]) encodeLocked() ([]byte, []string, error) {
	redacted, omitted := redactSecrets(c.value, c.secrets)
	out, err := json.MarshalIndent(typedEnvelope[T]{SchemaVersion: c.version, Data: redacted}, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return append(out, '\n'), omitted, nil
}
