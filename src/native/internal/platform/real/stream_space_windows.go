package real

import (
	"errors"
	"math"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// streamSpace resolves a volume GUID from the open directory handle. Volumes without a GUID fail closed.
func streamSpace(parent *os.File) (int64, error) {
	const volumeNameGUID = 1
	buf := make([]uint16, 32768)
	n, err := windows.GetFinalPathNameByHandle(windows.Handle(parent.Fd()), &buf[0], uint32(len(buf)), volumeNameGUID)
	if err != nil {
		return 0, err
	}
	if n == 0 || n >= uint32(len(buf)) {
		return 0, errors.New("cannot resolve directory volume")
	}
	final := windows.UTF16ToString(buf[:n])
	const prefix = `\\?\Volume{`
	if !strings.HasPrefix(final, prefix) {
		return 0, errors.New("directory has no volume GUID")
	}
	end := strings.Index(final[len(prefix):], `}\`)
	if end < 0 {
		return 0, errors.New("invalid directory volume GUID")
	}
	volume := final[:len(prefix)+end+2]
	path, err := windows.UTF16PtrFromString(volume)
	if err != nil {
		return 0, err
	}
	var available uint64
	if err := windows.GetDiskFreeSpaceEx(path, &available, nil, nil); err != nil {
		return 0, err
	}
	if available > math.MaxInt64 {
		return math.MaxInt64, nil
	}
	return int64(available), nil
}
