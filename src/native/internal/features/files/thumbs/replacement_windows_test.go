package thumbs

import (
	"errors"
	"syscall"
)

// MoveFileEx can reject replacement while the destination is open, including
// when the reader permits delete sharing. No replacement has occurred then.
func replacementBlockedByReader(err error) bool {
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED)
}
