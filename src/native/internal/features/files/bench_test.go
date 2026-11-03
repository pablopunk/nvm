package files

import (
	"context"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// benchQueries are the kinds of query a palette types: short prefixes, multi-word names, a miss and a kind filter.
var benchQueries = []Query{
	{Text: "ma"},
	{Text: "config"},
	{Text: "readme"},
	{Text: "invoice pl"},
	{Text: "zqx"},
	{Text: "budget", Kinds: []Kind{KindFile}},
	{Text: "img", Kinds: []Kind{KindImage}},
	{Text: "notes", SortBy: SortName, Order: OrderAsc},
}

// fixtureSnapshot builds the snapshot of n fixture paths, all of them: hidden and ignored names included, so the entry
// count is exactly n.
func fixtureSnapshot(n int) *Snapshot {
	paths := fixtures.Files(n)
	b := newBuilder(len(paths))
	for _, p := range paths {
		b.addRecord(record{dir: filepath.Dir(p), name: filepath.Base(p), size: 1024, modMs: 1_700_000_000_000, createdMs: 1_700_000_000_000})
	}
	return b.build(1, "bench")
}

// percentile returns the p-th percentile of sorted durations.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := min(int(float64(len(sorted)-1)*p+0.5), len(sorted)-1)
	return sorted[idx]
}

// BenchmarkSearch50k measures keystroke-to-final-results over a 50,000-entry snapshot, the B2 shape: each iteration runs
// one of the benchQueries to its final batch. The p50 and p95 of the iterations are reported as metrics.
func BenchmarkSearch50k(b *testing.B) {
	snap := fixtureSnapshot(50_000)
	env := searchEnv{home: "/home/user", roots: []string{"/home/user"}, skipDefaults: true}
	durations := make([]time.Duration, 0, b.N)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		q := benchQueries[i%len(benchQueries)]
		start := time.Now()
		for batch := range searchSnapshot(context.Background(), snap, q, env) {
			_ = batch
		}
		durations = append(durations, time.Since(start))
	}
	b.StopTimer()
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	b.ReportMetric(float64(percentile(durations, 0.50))/1e6, "p50-ms")
	b.ReportMetric(float64(percentile(durations, 0.95))/1e6, "p95-ms")
	b.ReportMetric(float64(percentile(durations, 0.99))/1e6, "p99-ms")
}

// BenchmarkFullScan50k measures a full walk of 50,000 fixture files through the fake file system, which is the cost of
// the first scan on a cold start before the platform port adds its own.
func BenchmarkFullScan50k(b *testing.B) {
	fsys := fake.NewFS()
	if err := fixtures.SeedFiles(fsys, fixtures.Files(50_000)); err != nil {
		b.Fatal(err)
	}
	cfg := scanConfig{maxDepth: 12, maxEntries: 60_000, includeHidden: true, statFiles: true, workers: scanWorkers}
	b.ReportAllocs()
	for range b.N {
		var counters scanCounters
		result, err := scanTrees(context.Background(), fsys, cfg, []scanStart{{dir: "/home/user", remaining: 12}}, &counters)
		if err != nil {
			b.Fatal(err)
		}
		if len(result.records) != 50_000 {
			b.Fatalf("scanned %d files, want 50000", len(result.records))
		}
	}
}

// TestMemoryFootprintOfTheIndex builds snapshots of 50,000 and 500,000 entries and reports what they hold in memory and
// on disk, per entry. Only the snapshot is measured: the path list it is built from is allocated before the baseline.
func TestMemoryFootprintOfTheIndex(t *testing.T) {
	for _, n := range []int{50_000, 500_000} {
		paths := fixtures.Files(n)
		runtime.GC()
		var before runtime.MemStats
		runtime.ReadMemStats(&before)

		b := newBuilder(len(paths))
		for _, p := range paths {
			b.addRecord(record{dir: filepath.Dir(p), name: filepath.Base(p), size: 1024, modMs: 1, createdMs: 1})
		}
		snap := b.build(1, "memory")
		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)

		heap := int64(after.HeapAlloc) - int64(before.HeapAlloc)
		encoded := len(encodeSnapshot(snap))
		perEntry := float64(heap) / float64(snap.Len())
		t.Logf("%d entries: heap %.1f MiB (%.1f bytes/entry; entry struct %d bytes, %d directories, %d extensions, %d name bytes); encoded %.1f MiB",
			snap.Len(), float64(heap)/(1<<20), perEntry, entrySize, len(snap.dirs), len(snap.exts), len(snap.names), float64(encoded)/(1<<20))
		if snap.Len() != n {
			t.Fatalf("snapshot holds %d entries, want %d", snap.Len(), n)
		}
		runtime.KeepAlive(snap)
		runtime.KeepAlive(paths)
	}
}

// TestSearchLatencyOverFiftyThousandEntries runs the benchQueries once over a 50,000-entry snapshot and logs the
// latency of each, so a run shows the B2 numbers without the benchmark harness.
func TestSearchLatencyOverFiftyThousandEntries(t *testing.T) {
	snap := fixtureSnapshot(50_000)
	env := searchEnv{home: "/home/user", roots: []string{"/home/user"}, skipDefaults: true}
	var durations []time.Duration
	for range 5 {
		for _, q := range benchQueries {
			start := time.Now()
			for batch := range searchSnapshot(context.Background(), snap, q, env) {
				_ = batch
			}
			durations = append(durations, time.Since(start))
		}
	}
	sorted := slices.Clone(durations)
	slices.Sort(sorted)
	t.Logf("50k entries, %d queries: p50 %.2f ms, p95 %.2f ms, p99 %.2f ms, max %.2f ms",
		len(durations), ms(percentile(sorted, 0.50)), ms(percentile(sorted, 0.95)), ms(percentile(sorted, 0.99)), ms(sorted[len(sorted)-1]))
}

// ms converts a duration to fractional milliseconds for a log line.
func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// TestFiftyThousandFilesIndexedFromTheFileSystem seeds 50,000 fixture files into the fake file system and indexes them
// through the real start-up path, reporting how long the first snapshot took.
func TestFiftyThousandFilesIndexedFromTheFileSystem(t *testing.T) {
	fsys := fake.NewFS()
	if err := fixtures.SeedFiles(fsys, fixtures.Files(50_000)); err != nil {
		t.Fatal(err)
	}
	changes := make(chan struct{}, 64)
	ix, err := New(Config{
		Home:               "/home/user",
		Roots:              []string{"/home/user"},
		MaxDepth:           12,
		MaxEntries:         60_000,
		IncludeHidden:      true,
		SkipDefaultIgnores: true,
		FS:                 fsys,
		Clock:              fake.NewClock(),
		OnChange: func() {
			select {
			case changes <- struct{}{}:
			default:
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	start := time.Now()
	if err := ix.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitFor(t, changes, func() bool { return ix.Status().Ready })
	elapsed := time.Since(start)
	st := ix.Status()
	if st.Entries != 50_000 || st.Truncated {
		t.Fatalf("first snapshot holds %d entries (truncated %v), want 50000", st.Entries, st.Truncated)
	}
	t.Logf("first snapshot of 50k files from the fake file system in %.0f ms (%d folders read)", ms(elapsed), st.ScannedDirs)
}
