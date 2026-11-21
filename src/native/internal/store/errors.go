package store

import (
	"errors"
	"fmt"
	"time"
)

// ErrClosed is returned by writes and subscriptions on a store that Close has shut down.
var ErrClosed = errors.New("store: closed")

// ErrReadOnly is returned by writes to a document that opened read-only: the file was written by a newer
// build, or it could not be read or migrated. The file is never overwritten in that state.
var ErrReadOnly = errors.New("store: document is read-only")

// ErrSecretInStore is returned by Save and Flush when a store:"secret" field held a value. The document is
// written without that field, a SecretOmitted notice is emitted, and the error tells the caller to move the
// secret to the keychain port.
var ErrSecretInStore = errors.New("store: a secret field was omitted from the file; secrets belong in the keychain")

// ErrLeaseHeld is returned by AcquireLease when another live instance holds the directory. A *LeaseHeldError
// wraps it with the holder's details.
var ErrLeaseHeld = errors.New("store: another instance holds the lease")

// ErrLeaseLost is returned by writes after another instance took the lease over. Writes stop; nothing is
// overwritten.
var ErrLeaseLost = errors.New("store: the lease was taken over by another instance")

// ErrLeaseReleased is returned by lease operations after Release.
var ErrLeaseReleased = errors.New("store: the lease was released")

// ErrMigration is returned when a migration step fails or produces data that does not match the document.
var ErrMigration = errors.New("store: migration failed")

// LeaseHeldError is returned by AcquireLease when a live holder owns the directory. It matches ErrLeaseHeld
// and carries what the holder recorded, so the UI can say which instance has the store open.
type LeaseHeldError struct {
	Holder Holder
}

// Error describes the holder by process id and start time. It never includes file content.
func (e *LeaseHeldError) Error() string {
	if e.Holder.PID == 0 {
		return "store: lease held by an owner whose record cannot be read"
	}
	return fmt.Sprintf("store: lease held by process %d since %s", e.Holder.PID, e.Holder.Started.Format(time.RFC3339))
}

// Is reports whether target is ErrLeaseHeld, so errors.Is works on the wrapped error.
func (e *LeaseHeldError) Is(target error) bool {
	return target == ErrLeaseHeld
}
