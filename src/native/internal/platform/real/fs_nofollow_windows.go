package real

import (
	"errors"
	"io"
	"os"

	"github.com/pablopunk/nvm/native/internal/platform"
	"golang.org/x/sys/windows"
)

// ReadFileNoFollow reads a bounded regular file without following its final reparse point.
func (osFS) ReadFileNoFollow(name string, max int64) ([]byte, error) {
	path, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(h), name)
	defer f.Close()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return nil, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, platform.ErrFileSymlink
	}
	kind, err := windows.GetFileType(h)
	if err != nil {
		return nil, err
	}
	if kind != windows.FILE_TYPE_DISK || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return nil, errors.New("not a bounded regular file")
	}
	size := int64(info.FileSizeHigh)<<32 | int64(info.FileSizeLow)
	if size > max {
		return nil, platform.ErrFileTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, platform.ErrFileTooLarge
	}
	return data, nil
}
