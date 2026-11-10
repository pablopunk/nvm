package real

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/platform"
	"io"
	"os"
	"path/filepath"
)

// stagePictureSource gives pathname-only OS providers a private copy of the authorized handle.
// Copying streams on a worker, never buffers the whole source, and stops with the request.
func stagePictureSource(ctx context.Context, source platform.ReadSource, name string) (string, func(), error) {
	info, err := source.Stat()
	if err != nil {
		return "", nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 {
		return "", nil, errors.New("picture source is not a regular file")
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp("", "nvm-picture-source-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	target := filepath.Join(dir, "source"+filepath.Ext(name))
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	n, copyErr := io.Copy(output, io.LimitReader(pictureSourceReader{ctx: ctx, source: source}, info.Size()+1))
	closeErr := output.Close()
	after, statErr := source.Stat()
	switch {
	case copyErr != nil:
		err = copyErr
	case closeErr != nil:
		err = closeErr
	case statErr != nil:
		err = statErr
	case n != info.Size() || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()):
		err = errors.New("picture source changed while being read")
	default:
		err = ctx.Err()
	}
	if err != nil {
		cleanup()
		return "", nil, err
	}
	return target, cleanup, nil
}

type pictureSourceReader struct {
	ctx    context.Context
	source io.Reader
}

func (r pictureSourceReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(data)
}
