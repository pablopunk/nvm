package usage

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/store"
)

// The memory directory of the persistence tests, inside the fake file system.
const testDir = "/cfg/nvm-native"

func newMemory(t *testing.T) (*Learner, *fake.Clock) {
	t.Helper()
	clock := fake.NewClock()
	return NewMemory(clock), clock
}

// TestBoostDecaysWithAge is the decay table: one use in the root, read back for another query so that only
// the usage and recency terms count. The usage term is six units; the recency term is 20 units less one per
// hour, so it is gone after twenty hours, and a day, a week and a month later only the usage term is left.
func TestBoostDecaysWithAge(t *testing.T) {
	tests := []struct {
		name string
		age  time.Duration
		want int
	}{
		{"just now", 0, 26},
		{"one hour", time.Hour, 25},
		{"half an hour keeps the whole unit", 30 * time.Minute, 25},
		{"one day", 24 * time.Hour, 6},
		{"seven days", 7 * 24 * time.Hour, 6},
		{"thirty days", 30 * 24 * time.Hour, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, clock := newMemory(t)
			l.Record("app:firefox", "fire")
			clock.Advance(tt.age)
			if got := l.Boost("app:firefox", "other"); got != tt.want {
				t.Fatalf("Boost after %s = %d, want %d", tt.age, got, tt.want)
			}
		})
	}
}

// TestUsageTermCapsAtNinety is the 1.x usageBoost cap: fifteen uses reach it, and more do not add to it.
func TestUsageTermCapsAtNinety(t *testing.T) {
	l, clock := newMemory(t)
	for i := 0; i < 40; i++ {
		l.Record("app:x", "")
	}
	clock.Advance(3 * 24 * time.Hour)
	if got := l.Boost("app:x", "zzz"); got != UsageCap {
		t.Fatalf("Boost after 40 uses long ago = %d, want %d", got, UsageCap)
	}
	if got := l.Uses("app:x"); got != 40 {
		t.Fatalf("Uses = %d, want 40", got)
	}
}

// TestRecencyTakesTheLatestUse checks that the recency term counts from the most recent use, as the 1.x
// effectiveLastUsed does: an old use and a fresh one give the fresh one's recency.
func TestRecencyTakesTheLatestUse(t *testing.T) {
	l, clock := newMemory(t)
	l.Record("app:x", "")
	clock.Advance(10 * time.Hour)
	l.Record("app:x", "")
	if got := l.Boost("app:x", "zzz"); got != 2*UsageWeight+RecencyCap {
		t.Fatalf("Boost right after the second use = %d, want %d", got, 2*UsageWeight+RecencyCap)
	}
}

// TestBoostIsPerQuery is the affinity rule: a row chosen for "cl" ranks higher for "cl" than for "c".
func TestBoostIsPerQuery(t *testing.T) {
	l, _ := newMemory(t)
	for i := 0; i < 3; i++ {
		l.Record("cmd:Clipboard History", "cl")
	}
	onCL := l.Boost("cmd:Clipboard History", "cl")
	onC := l.Boost("cmd:Clipboard History", "c")
	if onCL <= onC {
		t.Fatalf("Boost for cl = %d should exceed Boost for c = %d", onCL, onC)
	}
	if want := 3*UsageWeight + 3*UsageWeight + RecencyCap; onCL != want {
		t.Fatalf("Boost for cl = %d, want %d", onCL, want)
	}
	if want := 3*UsageWeight + RecencyCap; onC != want {
		t.Fatalf("Boost for c = %d, want %d (the usage and recency terms only)", onC, want)
	}
}

func TestQueriesAreNormalized(t *testing.T) {
	l, _ := newMemory(t)
	l.Record("app:x", "  Cl ")
	if l.Boost("app:x", "cl") != l.Boost("app:x", "CL") || l.Boost("app:x", "cl") <= l.Boost("app:x", "c") {
		t.Fatal("queries differing only in case and surrounding space should share their affinity")
	}
}

func TestUnusedAndEmptyIDsAreNotLearned(t *testing.T) {
	l, _ := newMemory(t)
	l.Record("", "x")
	if got := l.Boost("", "x"); got != 0 {
		t.Fatalf("Boost of the empty id = %d", got)
	}
	if got := l.Boost("app:never", "x"); got != 0 {
		t.Fatalf("Boost of an unused row = %d", got)
	}
	if got := l.Uses("app:never"); got != 0 {
		t.Fatalf("Uses of an unused row = %d", got)
	}
}

// TestMemoryIsBounded records more rows and queries than the bounds allow and checks that the least recently
// used go first.
func TestMemoryIsBounded(t *testing.T) {
	l, clock := newMemory(t)
	for i := 0; i < MaxItems+50; i++ {
		clock.Advance(time.Millisecond)
		l.Record(fmt.Sprintf("app:%04d", i), "")
	}
	if got := len(l.items); got != MaxItems {
		t.Fatalf("remembered rows = %d, want %d", got, MaxItems)
	}
	if got := l.Uses("app:0000"); got != 0 {
		t.Fatalf("the oldest row should be forgotten, Uses = %d", got)
	}
	if got := l.Uses(fmt.Sprintf("app:%04d", MaxItems+49)); got != 1 {
		t.Fatalf("the newest row should be kept, Uses = %d", got)
	}

	for i := 0; i < MaxQueriesPerItem+30; i++ {
		clock.Advance(time.Millisecond)
		l.Record("app:q", fmt.Sprintf("query %d", i))
	}
	if got := len(l.items["app:q"].Queries); got != MaxQueriesPerItem {
		t.Fatalf("remembered queries of one row = %d, want %d", got, MaxQueriesPerItem)
	}
	uses := MaxQueriesPerItem + 30
	forgotten := l.Boost("app:q", "query 0")
	if want := UsageCap + RecencyCap; forgotten != want {
		t.Fatalf("Boost of a forgotten query = %d, want %d (usage and recency only)", forgotten, want)
	}
	kept := l.Boost("app:q", fmt.Sprintf("query %d", uses-1))
	if want := UsageCap + UsageWeight + RecencyCap; kept != want {
		t.Fatalf("Boost of the newest query = %d, want %d", kept, want)
	}
}

func TestLongQueriesAreBounded(t *testing.T) {
	l, _ := newMemory(t)
	long := strings.Repeat("a", 500)
	l.Record("app:x", long)
	if _, ok := l.items["app:x"].Queries[long[:maxQueryRunes]]; !ok {
		t.Fatalf("a long query should be remembered by its first %d runes", maxQueryRunes)
	}
}

// TestBoostIsMonotonicInUsage is the property the ranking relies on: another use never lowers the boost of a
// row, for the query it was used with and for any other.
func TestBoostIsMonotonicInUsage(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	queries := []string{"", "c", "cl", "clip", "zzz"}
	for run := 0; run < 50; run++ {
		l, clock := newMemory(t)
		prev := map[string]int{}
		for step := 0; step < 80; step++ {
			clock.Advance(time.Duration(rng.Intn(3600)) * time.Second)
			query := queries[rng.Intn(len(queries))]
			l.Record("app:p", query)
			for _, q := range queries {
				got := l.Boost("app:p", q)
				if got < prev[q] {
					t.Fatalf("run %d step %d: Boost for %q fell from %d to %d after a use", run, step, q, prev[q], got)
				}
				prev[q] = got
			}
		}
	}
}

// TestBoostIsNonIncreasingWithAge is the other half: time alone never raises the boost of a row.
func TestBoostIsNonIncreasingWithAge(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	for run := 0; run < 50; run++ {
		l, clock := newMemory(t)
		for i := rng.Intn(20); i >= 0; i-- {
			l.Record("app:p", "cl")
			clock.Advance(time.Duration(rng.Intn(7200)) * time.Second)
		}
		prev := l.Boost("app:p", "cl")
		for step := 0; step < 60; step++ {
			clock.Advance(time.Duration(rng.Intn(4*3600)) * time.Second)
			got := l.Boost("app:p", "cl")
			if got > prev {
				t.Fatalf("run %d step %d: Boost rose from %d to %d with age", run, step, prev, got)
			}
			prev = got
		}
	}
}

// TestBoostIsReadSafelyWhileRecording runs recorders and readers together; under -race it proves the memory
// is guarded, and the counts prove no use was lost.
func TestBoostIsReadSafelyWhileRecording(t *testing.T) {
	l, _ := newMemory(t)
	const writers, uses = 8, 200
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < uses; i++ {
				l.Record(fmt.Sprintf("app:%d", w%3), "cl")
			}
		}(w)
	}
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < uses; i++ {
				_ = l.Boost("app:0", "cl")
				_ = l.Uses("app:1")
			}
		}()
	}
	wg.Wait()
	total := 0
	for i := 0; i < 3; i++ {
		total += l.Uses(fmt.Sprintf("app:%d", i))
	}
	if total != writers*uses {
		t.Fatalf("recorded %d uses, want %d", total, writers*uses)
	}
}

// TestLearnerSatisfiesTheSearchInterface keeps the usage package and the coordinator's contract together.
func TestLearnerSatisfiesTheSearchInterface(t *testing.T) {
	var _ search.Learner = NewMemory(fake.NewClock())
}

// openStore opens a store on a fresh fake file system; the debounce is an hour unless the test sets it.
func openStore(t *testing.T, files *fake.FS, clock *fake.Clock, opts ...store.Option) *store.Store {
	t.Helper()
	all := append([]store.Option{store.WithDebounce(time.Hour)}, opts...)
	s, err := store.Open(files, clock, testDir, all...)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestMemoryRoundTripsThroughTheStore records through a store, flushes, and reads the memory back with a new
// learner over the same files: the boosts and the counts must survive.
func TestMemoryRoundTripsThroughTheStore(t *testing.T) {
	files, clock := fake.NewFS(), fake.NewClock()
	s := openStore(t, files, clock)
	l, err := Open(s, clock, logx.Default())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	l.Record("app:firefox", "fire")
	l.Record("app:firefox", "fire")
	l.Record("cmd:Sleep", "")
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	clock.Advance(2 * time.Hour)
	want := l.Boost("app:firefox", "fire")

	reopened, err := Open(openStore(t, files, clock), clock, logx.Default())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := reopened.Boost("app:firefox", "fire"); got != want {
		t.Fatalf("Boost after the round trip = %d, want %d", got, want)
	}
	if got := reopened.Uses("cmd:Sleep"); got != 1 {
		t.Fatalf("Uses(cmd:Sleep) after the round trip = %d, want 1", got)
	}
}

// TestSavesAreDebounced checks that a use is written one debounce period after the last use, not at once.
func TestSavesAreDebounced(t *testing.T) {
	files, clock := fake.NewFS(), fake.NewClock()
	saved := make(chan string, 8)
	s := openStore(t, files, clock, store.WithDebounce(time.Minute), store.WithOnSaved(func(path string) { saved <- path }))
	l, err := Open(s, clock, logx.Default())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	l.Record("app:a", "")
	l.Record("app:b", "")
	select {
	case path := <-saved:
		t.Fatalf("written before the debounce period: %s", path)
	default:
	}
	clock.Advance(time.Minute)
	select {
	case <-saved:
	case <-time.After(10 * time.Second):
		t.Fatal("the memory was not written after the debounce period")
	}
	if n := len(saved); n != 0 {
		t.Fatalf("%d extra writes for two uses in one period", n)
	}
}

// TestCorruptFileIsQuarantined writes garbage where usage.json belongs. The store moves the file aside with a
// notice, and the learner starts empty and usable.
func TestCorruptFileIsQuarantined(t *testing.T) {
	files, clock := fake.NewFS(), fake.NewClock()
	files.Seed(testDir+"/usage.json", []byte("{this is not json"))
	s := openStore(t, files, clock)
	l, err := Open(s, clock, logx.Default())
	if err != nil {
		t.Fatalf("a quarantined file is not a load error: %v", err)
	}
	if got := l.Uses("app:a"); got != 0 {
		t.Fatalf("the learner should start empty, Uses = %d", got)
	}
	l.Record("app:a", "")
	if got := l.Boost("app:a", ""); got == 0 {
		t.Fatal("the learner should keep working after a quarantine")
	}
	select {
	case notice := <-s.Notices():
		if notice.Kind != store.Quarantined {
			t.Fatalf("notice kind = %v, want quarantined", notice.Kind)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no notice for the quarantined file")
	}
	moved := false
	for _, path := range files.Paths() {
		if strings.HasPrefix(path, testDir+"/usage.json.corrupt-") {
			moved = true
		}
	}
	if !moved {
		t.Fatalf("the corrupt file was not kept aside: %v", files.Paths())
	}
}

// TestOversizedFileIsBounded writes more rows than the bounds allow; loading keeps the most recent ones.
func TestOversizedFileIsBounded(t *testing.T) {
	items := map[string]item{}
	for i := 0; i < MaxItems+20; i++ {
		items[fmt.Sprintf("app:%d", i)] = item{Count: 1, LastUsed: int64(i + 1)}
	}
	if got := len(sanitize(items)); got != MaxItems {
		t.Fatalf("sanitized rows = %d, want %d", got, MaxItems)
	}
	if _, ok := sanitize(items)["app:0"]; ok {
		t.Fatal("the least recently used row should be dropped on load")
	}
}
