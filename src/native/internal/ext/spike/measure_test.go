//go:build spike

package spike

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// Run-time helpers shared by every measurement file.

// percentile returns the p-th percentile (0..100) of sorted durations using nearest rank.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(float64(len(sorted))*p/100.0+0.999999) - 1
	rank = max(0, min(rank, len(sorted)-1))
	return sorted[rank]
}

// summarize sorts the samples and returns p50, p95 and p99 in microseconds.
func summarize(samples []time.Duration) (p50, p95, p99 float64) {
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	us := func(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e3 }
	return us(percentile(sorted, 50)), us(percentile(sorted, 95)), us(percentile(sorted, 99))
}

// heapAlloc forces two collections and returns the live heap in bytes.
func heapAlloc() uint64 {
	runtime.GC()
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.HeapAlloc
}

// totalAllocated returns the cumulative bytes allocated by the process. Bomb tests use it because
// the memory an extension allocates may be unreachable again by the time a heap sample is taken.
func totalAllocated() uint64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.TotalAlloc
}

// measureLine prints one table row so that `go test -v` output can be pasted into the doc.
func measureLine(t *testing.T, format string, args ...any) {
	t.Helper()
	fmt.Printf("MEASURE "+format+"\n", args...)
}

// sourceLines counts the lines of an extension source.
func sourceLines(source string) int {
	return strings.Count(source, "\n")
}

// medianDuration runs fn n times and returns the median wall time.
func medianDuration(n int, fn func() error) (time.Duration, error) {
	samples := make([]time.Duration, 0, n)
	for range n {
		start := time.Now()
		if err := fn(); err != nil {
			return 0, err
		}
		samples = append(samples, time.Since(start))
	}
	slices.Sort(samples)
	return samples[len(samples)/2], nil
}

// requireNoError keeps the measurement tests short and readable.
func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
