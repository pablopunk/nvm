package real

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// procRoot is where Linux publishes process information.
const procRoot = "/proc"

// signalNumbers maps the port's signals onto Linux signal numbers.
var signalNumbers = map[platform.Signal]syscall.Signal{
	platform.SignalInterrupt: syscall.SIGINT,
	platform.SignalTerminate: syscall.SIGTERM,
	platform.SignalKill:      syscall.SIGKILL,
}

// List returns the current user's processes from procfs.
func (processes) List() ([]platform.Process, error) {
	return listProcessesUnder(procRoot, os.Getuid())
}

// Signal sends the signal to the process. A process that no longer exists returns ErrNotFound.
func (processes) Signal(pid int, sig platform.Signal) error {
	number, ok := signalNumbers[sig]
	if !ok {
		return fmt.Errorf("platform: unknown signal %q", sig)
	}
	err := syscall.Kill(pid, number)
	if errors.Is(err, syscall.ESRCH) {
		return platform.ErrNotFound
	}
	return err
}

// listProcessesUnder reads procfs-style directories under root and keeps the processes owned by uid.
// Entries that vanish mid-read, or are not numeric, are skipped.
func listProcessesUnder(root string, uid int) ([]platform.Process, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	pageKB := int64(os.Getpagesize() / 1024)
	var list []platform.Process
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		process, ok := readProcessEntry(filepath.Join(root, entry.Name()), pid, uid, pageKB)
		if ok {
			list = append(list, process)
		}
	}
	return list, nil
}

// readProcessEntry describes one process directory, reporting false when it is not the user's or it
// disappeared while being read.
func readProcessEntry(dir string, pid, uid int, pageKB int64) (platform.Process, bool) {
	info, err := os.Stat(dir)
	if err != nil {
		return platform.Process{}, false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != uid {
		return platform.Process{}, false
	}
	comm, err := os.ReadFile(filepath.Join(dir, "comm"))
	if err != nil {
		return platform.Process{}, false
	}
	statm, err := os.ReadFile(filepath.Join(dir, "statm"))
	if err != nil {
		return platform.Process{}, false
	}
	fields := strings.Fields(string(statm))
	if len(fields) < 2 {
		return platform.Process{}, false
	}
	pages, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return platform.Process{}, false
	}
	return platform.Process{PID: pid, Name: strings.TrimSpace(string(comm)), ResidentKB: pages * pageKB}, true
}
