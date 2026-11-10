package real

import (
	"github.com/pablopunk/nvm/native/internal/platform"
	"io"
	"os"
)

type readSource struct {
	*os.File
	identity string
}

func (s *readSource) Identity() string { return s.identity }

// ReadFileNoFollow reads only from the authorized regular-file descriptor.
func (f osFS) ReadFileNoFollow(name string, max int64) ([]byte, error) {
	source, err := f.OpenReadNoFollow(name)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > max {
		return nil, platform.ErrFileTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(source, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, platform.ErrFileTooLarge
	}
	return data, nil
}
