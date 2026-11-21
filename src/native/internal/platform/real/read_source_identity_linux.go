package real

import (
	"fmt"
	"io/fs"
	"syscall"
)

func sourceFileIdentity(info fs.FileInfo) string {
	stat := info.Sys().(*syscall.Stat_t)
	return fmt.Sprintf("%d:%d:%d:%d", stat.Dev, stat.Ino, stat.Ctim.Sec, stat.Ctim.Nsec)
}
