package perf

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// ErrUnsupported means a process measurement needs a Linux /proc, which this platform does not have.
var ErrUnsupported = errors.New("perf: process measurements need a Linux /proc")

// Hz is the kernel's USER_HZ, the unit of utime and stime in /proc/<pid>/stat. Linux builds that the
// rig runs on use 100; a kernel with another value would scale CPU percentages by that ratio.
const Hz = 100

// PID is a process ID.
type PID int

// statFields are the parts of /proc/<pid>/stat the measurements use.
type statFields struct {
	parent PID
	ticks  uint64 // utime + stime
}

// Live returns the /proc of this machine. It reports ErrUnsupported where /proc does not exist.
func Live() (fs.FS, error) {
	if runtime.GOOS != "linux" {
		return nil, ErrUnsupported
	}
	return os.DirFS("/proc"), nil
}

// ProcessTree returns root and every descendant of it that is running, root first. A child whose
// stat file disappears while the tree is read is skipped.
func ProcessTree(procFS fs.FS, root PID) ([]PID, error) {
	if _, err := readStat(procFS, root); err != nil {
		return nil, fmt.Errorf("perf: process %d: %w", root, err)
	}
	entries, err := fs.ReadDir(procFS, ".")
	if err != nil {
		return nil, fmt.Errorf("perf: read proc: %w", err)
	}
	children := map[PID][]PID{}
	for _, entry := range entries {
		pid, ok := numericPID(entry)
		if !ok {
			continue
		}
		stat, err := readStat(procFS, pid)
		if err != nil {
			continue
		}
		children[stat.parent] = append(children[stat.parent], pid)
	}
	var tree []PID
	seen := map[PID]bool{}
	stack := []PID{root}
	for len(stack) > 0 {
		pid := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[pid] {
			continue
		}
		seen[pid] = true
		tree = append(tree, pid)
		kids := children[pid]
		for i := len(kids) - 1; i >= 0; i-- {
			stack = append(stack, kids[i])
		}
	}
	return tree, nil
}

// PSSMiB sums the proportional set size of pids, in MiB, from /proc/<pid>/smaps_rollup. Processes
// that exit while the sum is taken count as zero; other read errors are returned.
func PSSMiB(procFS fs.FS, pids []PID) (float64, error) {
	var totalKB uint64
	for _, pid := range pids {
		kb, err := pssKB(procFS, pid)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return 0, err
		}
		totalKB += kb
	}
	return float64(totalKB) / 1024, nil
}

// CPUPercent is the CPU use of pids over window, as a percentage of one core. It reads utime and
// stime twice, window apart on clock, and divides the tick difference by the elapsed time and Hz.
// A process that starts during the window counts from its first tick; one that exits loses its
// last ticks, which only makes the figure lower.
func CPUPercent(procFS fs.FS, pids []PID, window time.Duration, clock platform.Clock) (float64, error) {
	if window <= 0 {
		return 0, errors.New("perf: the CPU window must be positive")
	}
	before, err := ticksOf(procFS, pids)
	if err != nil {
		return 0, err
	}
	start := clock.Now()
	clock.Sleep(window)
	after, err := ticksOf(procFS, pids)
	if err != nil {
		return 0, err
	}
	elapsed := clock.Now().Sub(start)
	if elapsed <= 0 {
		return 0, errors.New("perf: the clock did not advance over the CPU window")
	}
	var delta uint64
	for pid, ticks := range after {
		prev, known := before[pid]
		switch {
		case !known:
			delta += ticks
		case ticks >= prev:
			delta += ticks - prev
		}
	}
	return 100 * (float64(delta) / Hz) / elapsed.Seconds(), nil
}

// ticksOf returns utime+stime per running pid. Processes that are gone are left out.
func ticksOf(procFS fs.FS, pids []PID) (map[PID]uint64, error) {
	ticks := make(map[PID]uint64, len(pids))
	for _, pid := range pids {
		stat, err := readStat(procFS, pid)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		ticks[pid] = stat.ticks
	}
	return ticks, nil
}

func readStat(procFS fs.FS, pid PID) (statFields, error) {
	data, err := fs.ReadFile(procFS, fmt.Sprintf("%d/stat", pid))
	if err != nil {
		return statFields{}, err
	}
	return parseStat(string(data))
}

// parseStat reads the fields after the command name, which is in parentheses and may hold spaces.
// Field 4 (ppid) is index 1 after the closing parenthesis, field 14 (utime) index 11, field 15
// (stime) index 12.
func parseStat(line string) (statFields, error) {
	end := strings.LastIndexByte(line, ')')
	if end < 0 {
		return statFields{}, errors.New("perf: stat line has no command name")
	}
	fields := strings.Fields(line[end+1:])
	if len(fields) < 13 {
		return statFields{}, fmt.Errorf("perf: stat line has %d fields after the command, want at least 13", len(fields))
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return statFields{}, fmt.Errorf("perf: stat ppid: %w", err)
	}
	utime, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return statFields{}, fmt.Errorf("perf: stat utime: %w", err)
	}
	stime, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return statFields{}, fmt.Errorf("perf: stat stime: %w", err)
	}
	return statFields{parent: PID(ppid), ticks: utime + stime}, nil
}

func pssKB(procFS fs.FS, pid PID) (uint64, error) {
	file, err := procFS.Open(fmt.Sprintf("%d/smaps_rollup", pid))
	if err != nil {
		return 0, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		rest, found := strings.CutPrefix(scanner.Text(), "Pss:")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			break
		}
		return strconv.ParseUint(fields[0], 10, 64)
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("perf: %d/smaps_rollup has no Pss line", pid)
}

// numericPID reports whether a /proc entry is a process directory.
func numericPID(entry fs.DirEntry) (PID, bool) {
	if !entry.IsDir() {
		return 0, false
	}
	n, err := strconv.Atoi(entry.Name())
	if err != nil || n <= 0 {
		return 0, false
	}
	return PID(n), true
}
