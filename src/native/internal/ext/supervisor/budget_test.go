package supervisor

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// Budgets for the extension path. They catch order-of-magnitude regressions on shared CI machines, under -race too; see
// BenchmarkDormantFirstCall and BenchmarkSearchRoundTrip for the real numbers.
const (
	maxDormantFirstCall = 3 * time.Second
	maxSearchRoundTrip  = 20 * time.Millisecond // the B8 soft budget
)

func TestExtensionPathStaysWithinItsBudgets(t *testing.T) {
	requireUnixProcesses(t)
	r := newRuntime(t, "acme.good", nil)
	r.StartDormant([]string{protocol.MethodCommands, protocol.MethodSearch})
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	started := time.Now()
	if _, err := r.Call(ctx, protocol.MethodCommands, "", nil); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > maxDormantFirstCall {
		t.Errorf("a dormant first call took %s; the budget is %s", elapsed, maxDormantFirstCall)
	}
	samples := make([]time.Duration, 0, 200)
	for range 200 {
		began := time.Now()
		if _, err := r.Call(ctx, protocol.MethodSearch, "hello", nil); err != nil {
			t.Fatal(err)
		}
		samples = append(samples, time.Since(began))
	}
	slices.Sort(samples)
	if p95 := samples[len(samples)*95/100]; p95 > maxSearchRoundTrip {
		t.Errorf("search round trip p95 is %s; the budget is %s", p95, maxSearchRoundTrip)
	}
}
