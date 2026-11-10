package fake

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

const fakeStreamLimit int64 = 64 << 20
const fakeStreamCapacity int64 = 128 << 20
const OpWriteStream = "WriteStream"

var _ platform.StreamFiles = (*FS)(nil)

func (f *FS) reserveStream(ctx context.Context, size int64) (func(), error) {
	for {
		f.mu.Lock()
		if f.streamChanged == nil {
			f.streamChanged = make(chan struct{})
		}
		if err := ctx.Err(); err != nil {
			f.mu.Unlock()
			return nil, err
		}
		if size <= fakeStreamCapacity-f.streamUsed {
			f.streamUsed += size
			f.mu.Unlock()
			return func() {
				f.mu.Lock()
				f.streamUsed -= size
				close(f.streamChanged)
				f.streamChanged = make(chan struct{})
				f.mu.Unlock()
			}, nil
		}
		changed := f.streamChanged
		f.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		}
	}
}

func (f *FS) WriteStream(ctx context.Context, name string, source io.Reader, maxBytes int64, overwrite bool, admit func() error) (result platform.StreamFileResult, err error) {
	if !strings.HasPrefix(name, "/") || path.Base(name) == "." || source == nil || admit == nil || maxBytes < 0 || maxBytes > fakeStreamLimit {
		return result, fs.ErrInvalid
	}
	if err := f.faults.take(OpWriteStream); err != nil {
		return result, err
	}
	release, err := f.reserveStream(ctx, maxBytes)
	if err != nil {
		return result, err
	}
	defer release()
	key := cleanKey(name)
	f.mu.Lock()
	parent, err := f.lookupLocked(path.Dir(key))
	if err == nil && (!parent.dir || parent.link != "") {
		err = fs.ErrInvalid
	}
	f.mu.Unlock()
	if err != nil {
		return result, err
	}
	var data bytes.Buffer
	buf := make([]byte, 32<<10)
	for {
		if err = ctx.Err(); err != nil {
			return result, err
		}
		n, readErr := source.Read(buf[:min(int64(len(buf)), maxBytes+1-result.Bytes)])
		result.Bytes += int64(n)
		if result.Bytes > maxBytes {
			return result, platform.ErrFileTooLarge
		}
		if n > 0 {
			_, _ = data.Write(buf[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return result, readErr
		}
		if n == 0 {
			return result, io.ErrNoProgress
		}
	}
	if err = f.faults.take(OpWriteFile); err != nil {
		return result, err
	}
	if err = f.faults.take(OpSync); err != nil {
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
	if err = f.faults.take(OpRename); err != nil {
		return result, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	current, err := f.lookupLocked(path.Dir(key))
	if err != nil || current != parent {
		return result, fs.ErrNotExist
	}
	if existing, ok := f.nodes[key]; ok {
		if !overwrite {
			return result, fs.ErrExist
		}
		if existing.dir {
			return result, fs.ErrInvalid
		}
	}
	f.nodes[key] = &node{mode: 0600, data: data.Bytes()}
	f.kids = nil
	f.notifyLocked(platform.Event{Op: platform.EventWrite, Path: key})
	result.Published = true
	return result, nil
}
