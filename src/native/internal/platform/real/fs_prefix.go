package real

import (
	"errors"
	"io"
	"os"
)

// readFilePrefix opens name, reads at most limit leading bytes and reports the size the file had when it was
// opened. A file that shrinks while it is read returns the bytes that were there.
func readFilePrefix(name string, limit int64) ([]byte, int64, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, 0, err
	}
	size := info.Size()
	want := size
	if limit < want {
		want = limit
	}
	if want < 0 {
		want = 0
	}
	buf := make([]byte, want)
	n, err := io.ReadFull(file, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, 0, err
	}
	return buf[:n], size, nil
}
