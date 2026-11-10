package real

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/platform"
)

const streamFileLimit int64 = 64 << 20

var _ platform.StreamFiles = osFS{}

// WriteStream stages privately under a pinned parent and publishes only after authority is revalidated.
func (osFS) WriteStream(ctx context.Context, name string, source io.Reader, maxBytes int64, overwrite bool, admit func() error) (result platform.StreamFileResult, err error) {
	if !filepath.IsAbs(name) || filepath.Base(name) == "." || maxBytes < 0 || maxBytes > streamFileLimit || source == nil || admit == nil {
		return result, fs.ErrInvalid
	}
	dir, base := filepath.Dir(name), filepath.Base(name)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return result, err
	}
	release, err := pictureStaging.reserve(ctx, maxBytes+1, func() (int64, error) { return spaceAt(dir) })
	if err != nil {
		_ = root.Close()
		return result, err
	}
	var token [16]byte
	if _, err = rand.Read(token[:]); err != nil {
		release()
		_ = root.Close()
		return result, err
	}
	temp := ".nvm-stream-" + hex.EncodeToString(token[:])
	file, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		release()
		_ = root.Close()
		return result, err
	}
	cleanup := &pictureCleanup{budget: pictureStaging, path: temp, release: release, remove: func(n string) error {
		e := root.Remove(n)
		if errors.Is(e, fs.ErrNotExist) {
			e = nil
		}
		if e == nil {
			_ = root.Close()
		}
		return e
	}}
	defer func() {
		if e := cleanup.attempt(); e != nil {
			err = errors.Join(err, e)
		}
	}()
	result.Bytes, err = copyStream(ctx, file, source, maxBytes)
	if err == nil {
		err = file.Sync()
	}
	if e := file.Close(); err == nil {
		err = e
	}
	if err != nil {
		return result, err
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if err = admit(); err != nil {
		return result, err
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if overwrite {
		err = root.Rename(temp, base)
	} else {
		err = root.Link(temp, base)
	}
	if err != nil {
		return result, err
	}
	result.Published = true
	return result, nil
}

func copyStream(ctx context.Context, dst io.Writer, source io.Reader, maxBytes int64) (int64, error) {
	buf := make([]byte, 32<<10)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, readErr := source.Read(buf[:min(int64(len(buf)), maxBytes+1-total)])
		if n > 0 {
			total += int64(n)
			if total > maxBytes {
				return total, platform.ErrFileTooLarge
			}
			written, err := dst.Write(buf[:n])
			if err != nil {
				return total, err
			}
			if written != n {
				return total, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return total, nil
		}
		if readErr != nil {
			return total, readErr
		}
		if n == 0 {
			return total, io.ErrNoProgress
		}
	}
}
