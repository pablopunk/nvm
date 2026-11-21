package perf

import (
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// statLine is a /proc/<pid>/stat line with the given parent and CPU ticks. Fields 14 and 15 are
// utime and stime; the rest is filler the parser must skip.
func statLine(pid, parent int, utime, stime uint64) string {
	return fmt.Sprintf("%d (worker proc) S %d 1 1 0 -1 0 0 0 0 0 %d %d 0 0 20 0 1 0 1 0 0\n", pid, parent, utime, stime)
}

// smapsRollup is a /proc/<pid>/smaps_rollup with the given proportional set size in kB.
func smapsRollup(pssKB int) []byte {
	return []byte(fmt.Sprintf("Rss:          %d kB\nPss:          %d kB\nPss_Anon:     %d kB\n", pssKB*2, pssKB, pssKB))
}

// fakeProc is a /proc with a process tree 100 -> {101 -> 102, 103}, an unrelated process 200 and a
// non-process entry.
func fakeProc() fstest.MapFS {
	return fstest.MapFS{
		"100/stat":          {Data: []byte(statLine(100, 1, 500, 300))},
		"100/smaps_rollup":  {Data: smapsRollup(2048)},
		"101/stat":          {Data: []byte(statLine(101, 100, 100, 0))},
		"101/smaps_rollup":  {Data: smapsRollup(1024)},
		"102/stat":          {Data: []byte(statLine(102, 101, 0, 40))},
		"102/smaps_rollup":  {Data: smapsRollup(512)},
		"103/stat":          {Data: []byte(statLine(103, 100, 7, 3))},
		"103/smaps_rollup":  {Data: smapsRollup(256)},
		"200/stat":          {Data: []byte(statLine(200, 1, 9999, 9999))},
		"200/smaps_rollup":  {Data: smapsRollup(4096)},
		"self/stat":         {Data: []byte(statLine(1, 0, 1, 1))},
		"self/smaps_rollup": {Data: smapsRollup(1)},
		"100/fd/0":          {Data: []byte("")},
		"100/fd/1":          {Data: []byte("")},
		"100/fd/2":          {Data: []byte("")},
	}
}

func TestCountFDsCountsDescriptorEntries(t *testing.T) {
	n, err := CountFDs(fakeProc(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("CountFDs = %d, want 3", n)
	}
}

func TestProcessTreeListsRootAndDescendants(t *testing.T) {
	tree, err := ProcessTree(fakeProc(), 100)
	if err != nil {
		t.Fatal(err)
	}
	want := []PID{100, 101, 102, 103}
	if !reflect.DeepEqual(tree, want) {
		t.Fatalf("ProcessTree(100) = %v, want %v (root first, unrelated 200 left out)", tree, want)
	}
}

func TestProcessTreeOfLeafIsItself(t *testing.T) {
	tree, err := ProcessTree(fakeProc(), 102)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tree, []PID{102}) {
		t.Fatalf("ProcessTree(102) = %v, want [102]", tree)
	}
}

func TestProcessTreeRejectsRootThatIsNotRunning(t *testing.T) {
	_, err := ProcessTree(fakeProc(), 999)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ProcessTree(999) error = %v, want one wrapping fs.ErrNotExist", err)
	}
}

func TestProcessTreeSkipsChildrenThatExitDuringTheRead(t *testing.T) {
	procFS := fakeProc()
	procFS["104/stat"] = &fstest.MapFile{Data: []byte("garbage")}
	tree, err := ProcessTree(procFS, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tree, []PID{100, 101, 102, 103}) {
		t.Fatalf("ProcessTree with an unreadable child = %v", tree)
	}
}

func TestPSSMiBSumsTreeAndCountsExitedProcessesAsZero(t *testing.T) {
	mib, err := PSSMiB(fakeProc(), []PID{100, 101, 102, 103, 999})
	if err != nil {
		t.Fatal(err)
	}
	// (2048 + 1024 + 512 + 256) kB = 3840 kB = 3.75 MiB; pid 999 does not exist.
	if mib != 3.75 {
		t.Fatalf("PSSMiB = %v MiB, want 3.75", mib)
	}
}

func TestPSSMiBReportsMissingPssLine(t *testing.T) {
	procFS := fakeProc()
	procFS["100/smaps_rollup"] = &fstest.MapFile{Data: []byte("Rss: 10 kB\n")}
	if _, err := PSSMiB(procFS, []PID{100}); err == nil {
		t.Fatal("PSSMiB without a Pss line returned no error")
	}
}

// stepClock is a platform.Clock whose Sleep runs a hook first and then advances time, so a test can
// change the fake /proc between the two samples of CPUPercent.
type stepClock struct {
	platform.Clock
	now     time.Time
	onSleep func()
}

func (c *stepClock) Now() time.Time { return c.now }

func (c *stepClock) Sleep(d time.Duration) {
	if c.onSleep != nil {
		c.onSleep()
	}
	c.now = c.now.Add(d)
}

func TestCPUPercentFromTickDeltaOverWallTime(t *testing.T) {
	procFS := fakeProc()
	clock := &stepClock{now: time.Unix(0, 0)}
	clock.onSleep = func() {
		// Over the 2 s window, pid 100 used 100 ticks (1 s of CPU) and pid 101 used 50 ticks.
		procFS["100/stat"] = &fstest.MapFile{Data: []byte(statLine(100, 1, 500+80, 300+20))}
		procFS["101/stat"] = &fstest.MapFile{Data: []byte(statLine(101, 100, 100+50, 0))}
	}
	pct, err := CPUPercent(procFS, []PID{100, 101}, 2*time.Second, clock)
	if err != nil {
		t.Fatal(err)
	}
	// 150 ticks / 100 Hz = 1.5 CPU-seconds over 2 s of wall time = 75 percent of one core.
	if pct != 75 {
		t.Fatalf("CPUPercent = %v, want 75", pct)
	}
}

func TestCPUPercentCountsProcessesThatStartInTheWindow(t *testing.T) {
	procFS := fakeProc()
	clock := &stepClock{now: time.Unix(0, 0)}
	clock.onSleep = func() {
		procFS["104/stat"] = &fstest.MapFile{Data: []byte(statLine(104, 100, 30, 20))}
	}
	pct, err := CPUPercent(procFS, []PID{100, 104}, time.Second, clock)
	if err != nil {
		t.Fatal(err)
	}
	if pct != 50 {
		t.Fatalf("CPUPercent with a new child = %v, want 50", pct)
	}
}

func TestCPUPercentRejectsNonPositiveWindow(t *testing.T) {
	if _, err := CPUPercent(fakeProc(), []PID{100}, 0, &stepClock{now: time.Unix(0, 0)}); err == nil {
		t.Fatal("CPUPercent with a zero window returned no error")
	}
}

func TestParseStatHandlesSpacesAndParenthesesInCommandName(t *testing.T) {
	stat, err := parseStat("42 (my (odd) name) R 7 1 1 0 -1 0 0 0 0 0 11 22 0 0 20 0 1 0 1 0 0\n")
	if err != nil {
		t.Fatal(err)
	}
	if stat.parent != 7 || stat.ticks != 33 {
		t.Fatalf("parseStat = parent %d ticks %d, want parent 7 ticks 33", stat.parent, stat.ticks)
	}
}

func TestParseStatRejectsTruncatedLine(t *testing.T) {
	if _, err := parseStat("42 (short) S 1 1"); err == nil {
		t.Fatal("parseStat on a truncated line returned no error")
	}
}

func TestPercentileUsesNearestRank(t *testing.T) {
	samples := []float64{50, 10, 40, 20, 30}
	cases := []struct {
		p    float64
		want float64
	}{
		{0, 10}, {20, 10}, {50, 30}, {95, 50}, {100, 50},
	}
	for _, tc := range cases {
		if got := Percentile(samples, tc.p); got != tc.want {
			t.Errorf("Percentile(p=%v) = %v, want %v", tc.p, got, tc.want)
		}
	}
	if samples[0] != 50 {
		t.Fatal("Percentile reordered its input")
	}
	if got := Percentile(nil, 95); got != 0 {
		t.Fatalf("Percentile of no samples = %v, want 0", got)
	}
}

func TestMedianOfEvenAndOddCounts(t *testing.T) {
	if got := Median([]float64{3, 1, 2}); got != 2 {
		t.Fatalf("Median odd = %v, want 2", got)
	}
	if got := Median([]float64{4, 1, 3, 2}); got != 2.5 {
		t.Fatalf("Median even = %v, want 2.5", got)
	}
}

func TestReadCountersReportsThisProcess(t *testing.T) {
	counters := ReadCounters()
	if counters.Goroutines < 1 {
		t.Fatalf("goroutines = %d", counters.Goroutines)
	}
	if counters.HeapAlloc == 0 {
		t.Fatal("heapAlloc = 0")
	}
	if counters.FDs != -1 && counters.FDs < 3 {
		t.Fatalf("fds = %d, want at least the standard three or -1", counters.FDs)
	}
}

func TestLiveReturnsProcOrUnsupported(t *testing.T) {
	procFS, err := Live()
	if err != nil {
		if !errors.Is(err, ErrUnsupported) {
			t.Fatalf("Live error = %v, want ErrUnsupported", err)
		}
		return
	}
	if _, err := fs.Stat(procFS, "self"); err != nil {
		t.Fatalf("Live returned a /proc without self: %v", err)
	}
}
