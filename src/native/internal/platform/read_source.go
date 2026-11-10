package platform

import (
	"context"
	"errors"
	"io"
	"io/fs"
)

// ReadSource is one regular file opened for reading without following its final symlink.
// Identity names the underlying file, not its pathname. Callers close it on every path.
type ReadSource interface {
	io.ReadSeekCloser
	Stat() (fs.FileInfo, error)
	Identity() string
}

// FileSources opens source files before serving fresh or cached derived content.
type FileSources interface {
	OpenReadNoFollow(name string) (ReadSource, error)
}

// SourceFilePictures renders an already authorized source rather than reopening an unchecked pathname.
type SourceFilePictures interface {
	FilePictureFromSource(ctx context.Context, source ReadSource, name string, edge int) ([]byte, error)
}

// ErrSourceReadDenied distinguishes a permission error from the original source, not a staging/cache write.
var ErrSourceReadDenied = errors.New("picture source read access denied")
