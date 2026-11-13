package platform

import (
	"context"
	"io"
)

// StreamFiles writes a bounded stream through private staging beside an absolute destination.
// Implementations pin the destination directory, bound transient disk and memory, and remove staging
// on every path (retaining failed cleanup for retry). Existing destinations survive failed transfers.
type StreamFiles interface {
	// WriteStream reads at most maxBytes+1 to detect oversize input without truncating a successful
	// result. It checks cancellation and admit after writing/flushing, immediately before publication.
	// When overwrite is false, publication must refuse a destination created during the transfer.
	// admit is host-owned authority revalidation, never a callback supplied by an extension.
	WriteStream(ctx context.Context, path string, source io.Reader, maxBytes int64, overwrite bool, admit func() error) (StreamFileResult, error)
}

// StreamFileResult distinguishes bytes transferred from publication, including errors after publication.
type StreamFileResult struct {
	Bytes     int64
	Published bool
}
