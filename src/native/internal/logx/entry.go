package logx

import (
	"encoding/json"
	"time"
)

// F holds the structured fields of one entry. Values are normalized to JSON-compatible data and redacted
// before they are stored or written, so callers may pass errors, structs and maps directly.
type F map[string]any

// Entry is one redacted record, as kept in the ring buffer and written as one JSON line.
type Entry struct {
	Time    time.Time
	Level   Level
	Message string
	Fields  map[string]any
}

// jsonLine is the on-disk shape of an entry: one object per line, fields nested under "fields" so they
// can never collide with time, level or msg.
type jsonLine struct {
	Time    string         `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"msg"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// encode returns the entry as one JSON line ending in a newline.
func (e Entry) encode() ([]byte, error) {
	data, err := json.Marshal(jsonLine{
		Time:    e.Time.UTC().Format(time.RFC3339Nano),
		Level:   e.Level.String(),
		Message: e.Message,
		Fields:  e.Fields,
	})
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
