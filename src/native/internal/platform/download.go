package platform

import (
	"context"
	"errors"
)

// ErrChecksum is a download whose content does not match the checksum it was asked for. The partial file is removed.
var ErrChecksum = errors.New("the downloaded file does not match its checksum")

// FileDownloader is the intent "fetch a large file to disk", for content too big to hold in memory, such as an
// on-device model. Every OS supports it, subject to network state.
type FileDownloader interface {
	// Download fetches url into dest and checks its SHA-256 against sha256 (lowercase hex) before dest appears. A
	// download that stopped part way resumes where it stopped. progress hears the bytes on disk so far.
	Download(ctx context.Context, url, dest, sha256 string, progress func(done int64)) error
}
