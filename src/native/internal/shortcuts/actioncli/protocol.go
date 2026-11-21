package actioncli

import (
	"errors"
	"fmt"
	"unicode"
)

// protocolVersion is the request version a running instance accepts.
const protocolVersion = 1

// maxMessageBytes caps a request or a response read from the socket.
const maxMessageBytes = 16 << 10

// maxActionIDBytes caps an action ID; registered IDs are far shorter.
const maxActionIDBytes = 1024

// Request asks the running instance to run one registered action.
type Request struct {
	Version int    `json:"v"`
	Action  string `json:"action"`
}

// Outcome is what the running instance did with a request.
type Outcome string

const (
	// Completed: a noninteractive action ran to its end.
	Completed Outcome = "completed"
	// Started: a noninteractive action started; it reports its own result when it ends.
	Started Outcome = "started"
	// Opened: the action needs the palette, which is now open on it.
	Opened Outcome = "opened"
	// Rejected: no registered action has the ID, or it is unavailable now; nothing ran.
	Rejected Outcome = "rejected"
	// Failed: the action was found but could not run.
	Failed Outcome = "failed"
)

// Response is the running instance's answer to one Request.
type Response struct {
	Outcome Outcome `json:"outcome"`
	Message string  `json:"message,omitempty"`
}

// ErrInvalidActionID reports an action ID that no registered action could have.
var ErrInvalidActionID = errors.New("invalid action ID")

// ValidateActionID refuses an empty or oversized ID and one with control characters, so the ID can be logged and shown
// as it is.
func ValidateActionID(id string) error {
	if id == "" || len(id) > maxActionIDBytes {
		return fmt.Errorf("%w: it must be 1 to %d bytes", ErrInvalidActionID, maxActionIDBytes)
	}
	for _, r := range id {
		if unicode.IsControl(r) || r == unicode.ReplacementChar {
			return fmt.Errorf("%w: it has a control character", ErrInvalidActionID)
		}
	}
	return nil
}
