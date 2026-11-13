package real

import (
	"golang.org/x/sys/windows"
	"math"
	"os"
)

func pictureTemporarySpace() (int64, error) {
	return spaceAt(os.TempDir())
}

func spaceAt(dir string) (int64, error) {
	path, err := windows.UTF16PtrFromString(dir)
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
