//go:build rig

package rigtest

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/perf"
)

// soakDuration is how long the B7 loop runs. The full soak in PLAN.md is 30 minutes; CI runs the
// default, and the 30-minute run is `go test -args -soak=30m`.
var soakDuration = flag.Duration("soak", 2*time.Minute, "how long the B7 soak loop types, searches and dismisses the palette")

// The assertions of B7. The first soakWarmupCycles cycles are not judged, and the baseline is the
// median of the next five samples; the end is the median of the last five. Slacks are small on
// purpose: the palette's caches may hold a few goroutines and descriptors, but nothing per cycle.
const (
	soakWarmupCycles   = 3
	soakBaselineCount  = 5
	soakFinalCount     = 5
	soakMinSamples     = soakWarmupCycles + soakBaselineCount + soakFinalCount + 2
	soakGoroutineSlack = 5
	soakFDSlack        = 5
	soakHeapGrowthMiB  = 8
	// soakRisingLimit is the fraction of samples at which the heap rose above the one before. A
	// leak grows at nearly every sample; a healthy heap rises and falls about evenly.
	soakRisingLimit = 0.9
)

// soakQueries are typed in turn, one per cycle: prefixes that match applications and clipboard
// text, a calculation, a phrase with no match, and a word that matches a command.
var soakQueries = []string{"cl", "clip", "2+2", "show the time", "notes", "calc 12*3"}

// measureSoak is B7. It runs the app with the counters endpoint and loops: summon the palette, type
// a query, clear it, dismiss. After each cycle it reads the counters and checks that goroutines,
// descriptors and the post-collection heap stay within their slack.
func measureSoak(t *testing.T, rec *perfRecorder) {
	addr := freeLoopbackAddr(t)
	app, _ := launchPerfApp(t, addr)
	rec.noteRenderer(app.Log())
	started := time.Now()
	var samples []perf.Counters
	for cycle := 0; time.Since(started) < *soakDuration; cycle++ {
		soakCycle(t, app, cycle)
		samples = append(samples, fetchCounters(t, addr))
	}
	elapsed := time.Since(started)
	if len(samples) < soakMinSamples {
		t.Fatalf("the soak ran %d cycles in %s; it needs at least %d to judge growth: raise -soak", len(samples), elapsed.Round(time.Second), soakMinSamples)
	}
	judgeSoak(t, rec, samples, elapsed)
}

// soakCycle is one open-search-close cycle. Each step waits for the palette to show its state, so a
// cycle that leaves the palette in a hidden or stuck state fails here and not in the counters.
func soakCycle(t *testing.T, app *App, cycle int) {
	t.Helper()
	must(t, app.SummonPalette())
	query := soakQueries[cycle%len(soakQueries)]
	must(t, app.Type(query))
	waitSearchValue(t, app, query)
	must(t, app.Key("Escape"))
	waitSearchValue(t, app, "")
	must(t, app.DismissWithEscape())
}

func fetchCounters(t *testing.T, addr string) perf.Counters {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + addr + perf.DebugPath)
	if err != nil {
		t.Fatalf("counters endpoint: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("counters endpoint answered %s", resp.Status)
	}
	var counters perf.Counters
	if err := json.NewDecoder(resp.Body).Decode(&counters); err != nil {
		t.Fatalf("counters endpoint: %v", err)
	}
	return counters
}

// judgeSoak records the B7 measurements and fails the test for growth beyond the slacks.
func judgeSoak(t *testing.T, rec *perfRecorder, samples []perf.Counters, elapsed time.Duration) {
	t.Helper()
	baseline := samples[soakWarmupCycles : soakWarmupCycles+soakBaselineCount]
	final := samples[len(samples)-soakFinalCount:]

	goroutineDelta := medianOf(final, func(c perf.Counters) float64 { return float64(c.Goroutines) }) -
		medianOf(baseline, func(c perf.Counters) float64 { return float64(c.Goroutines) })
	fdDelta, fdNote := descriptorDelta(baseline, final)
	heapGrowthMiB := (medianOf(final, heapBytes) - medianOf(baseline, heapBytes)) / bytesPerMiB
	rising := risingFraction(samples[soakWarmupCycles:])
	heapMiB := make([]float64, len(samples))
	for i, c := range samples {
		heapMiB[i] = heapBytes(c) / bytesPerMiB
	}

	note := fmt.Sprintf("%d cycles over %s; baseline is the median of cycles %d-%d, end the median of the last %d",
		len(samples), elapsed.Round(time.Second), soakWarmupCycles+1, soakWarmupCycles+soakBaselineCount, soakFinalCount)
	rec.set("B7.goroutine_delta", goroutineDelta, "goroutines", nil, note)
	rec.set("B7.fd_delta", fdDelta, "fds", nil, note+fdNote)
	rec.set("B7.heap_growth_mib", heapGrowthMiB, "MiB", nil, "post-collection heap, end minus baseline: "+note)
	rec.set("B7.heap_rising_fraction", rising, "fraction", heapMiB, "share of samples where the heap rose; samples are post-collection heap MiB")
	rec.set("B7.cycles", float64(len(samples)), "cycles", nil, "information")
	rec.set("B7.duration_s", elapsed.Seconds(), "s", nil, "information")

	if goroutineDelta > soakGoroutineSlack {
		t.Errorf("goroutines grew by %.0f over the soak (slack %d)", goroutineDelta, soakGoroutineSlack)
	}
	if fdDelta > soakFDSlack {
		t.Errorf("open descriptors grew by %.0f over the soak (slack %d)", fdDelta, soakFDSlack)
	}
	if heapGrowthMiB > soakHeapGrowthMiB {
		t.Errorf("post-collection heap grew by %.1f MiB over the soak (slack %d MiB)", heapGrowthMiB, soakHeapGrowthMiB)
	}
	if rising > soakRisingLimit {
		t.Errorf("the heap rose in %.0f%% of samples: it grows monotonically (limit %.0f%%)", rising*100, soakRisingLimit*100)
	}
	t.Logf("soak: goroutines %+.0f, fds %+.0f, heap %+.1f MiB, rising %.0f%% over %d cycles",
		goroutineDelta, fdDelta, heapGrowthMiB, rising*100, len(samples))
}

func heapBytes(c perf.Counters) float64 { return float64(c.HeapAlloc) }

func medianOf(counters []perf.Counters, value func(perf.Counters) float64) float64 {
	values := make([]float64, len(counters))
	for i, c := range counters {
		values[i] = value(c)
	}
	return perf.Median(values)
}

// descriptorDelta is the change in open descriptors between the two windows. A platform that cannot
// count them reports -1 in every sample; the delta is then zero and the note says so.
func descriptorDelta(baseline, final []perf.Counters) (float64, string) {
	for _, c := range append(append([]perf.Counters{}, baseline...), final...) {
		if c.FDs < 0 {
			return 0, "; descriptors not counted on this platform"
		}
	}
	return medianOf(final, func(c perf.Counters) float64 { return float64(c.FDs) }) -
		medianOf(baseline, func(c perf.Counters) float64 { return float64(c.FDs) }), ""
}

// risingFraction is the share of consecutive pairs in which the heap went up.
func risingFraction(samples []perf.Counters) float64 {
	if len(samples) < 2 {
		return 0
	}
	rising := 0
	for i := 1; i < len(samples); i++ {
		if samples[i].HeapAlloc > samples[i-1].HeapAlloc {
			rising++
		}
	}
	return float64(rising) / float64(len(samples)-1)
}
