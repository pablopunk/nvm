package perf

import (
	"io/fs"
	"os"
	"runtime"
	"strconv"
)

// DebugPath is the URL the counters are served on in perfdebug builds. It is defined here, not in the
// perfdebug file, so the rig's test code compiles without that tag.
const DebugPath = "/debug/perf"

// Counters are the process counters the soak test samples through the debug endpoint (budget B7).
type Counters struct {
	Goroutines int    `json:"goroutines"`
	FDs        int    `json:"fds"`       // -1 when the platform cannot count them
	HeapAlloc  uint64 `json:"heapAlloc"` // bytes, measured after a collection
}

// ReadCounters reads this process's counters. The heap is read after a collection, so growth between
// samples is retained memory and not garbage waiting for the next cycle.
func ReadCounters() Counters {
	runtime.GC()
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	return Counters{
		Goroutines: runtime.NumGoroutine(),
		FDs:        openFDs(),
		HeapAlloc:  memory.HeapAlloc,
	}
}

// openFDs counts this process's open file descriptors, or returns -1.
func openFDs() int {
	procFS, err := Live()
	if err != nil {
		return -1
	}
	n, err := CountFDs(procFS, PID(os.Getpid()))
	if err != nil {
		return -1
	}
	return n
}

// CountFDs counts the entries of /proc/<pid>/fd.
func CountFDs(procFS fs.FS, pid PID) (int, error) {
	entries, err := fs.ReadDir(procFS, strconv.Itoa(int(pid))+"/fd")
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}
