package swifthelper

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Codes are the envelope codes the helper prints. Only "ok" carries data.
const (
	CodeOK          = "ok"
	CodeDenied      = "denied"
	CodeNoWindow    = "no-window"
	CodeTargetLost  = "target-lost"
	CodeUnsupported = "unsupported"
	CodeUnapplied   = "unapplied"
	CodeError       = "error"
)

// ErrUnreadable is returned when the helper's stdout is not an envelope with a known code.
var ErrUnreadable = errors.New("the window helper returned an unreadable response")

// Envelope is one helper answer.
type Envelope struct {
	Code    string          `json:"code"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

var knownCodes = map[string]platform.WindowErrorCode{
	CodeDenied:      platform.WindowErrorDenied,
	CodeNoWindow:    platform.WindowErrorNoWindow,
	CodeTargetLost:  platform.WindowErrorTargetLost,
	CodeUnsupported: platform.WindowErrorUnsupported,
	CodeUnapplied:   platform.WindowErrorUnapplied,
	CodeError:       platform.WindowErrorFailed,
}

// ParseEnvelope decodes the helper's stdout. Whatever the process exit status, the envelope is what counts.
func ParseEnvelope(stdout []byte) (Envelope, error) {
	var envelope Envelope
	if err := json.Unmarshal(stdout, &envelope); err != nil {
		return Envelope{}, ErrUnreadable
	}
	if envelope.Code != CodeOK {
		if _, known := knownCodes[envelope.Code]; !known {
			return Envelope{}, ErrUnreadable
		}
	}
	return envelope, nil
}

// Err returns the platform error for a non-ok envelope, and nil for an ok one.
func (e Envelope) Err() error {
	if e.Code == CodeOK {
		return nil
	}
	return &platform.WindowError{Code: knownCodes[e.Code], Message: e.Message}
}

// Decode reads the envelope's data into T. An ok envelope without data is unreadable.
func Decode[T any](e Envelope) (T, error) {
	var value T
	if len(e.Data) == 0 {
		return value, ErrUnreadable
	}
	if err := json.Unmarshal(e.Data, &value); err != nil {
		return value, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	return value, nil
}
