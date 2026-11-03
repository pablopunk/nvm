package store

import (
	"encoding/json"
	"runtime"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestHeartbeatLoopRenewsOnItsTicks(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	lease := mustAcquire(t, files, clock, WithHeartbeat(10*time.Second, 30*time.Second))
	before := recordHeartbeat(t, files)
	clock.Advance(10 * time.Second)
	waitFor(t, func() bool { return recordHeartbeat(t, files) != before })
	if err := lease.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

// recordHeartbeat returns the heartbeat time stored in the lock record.
func recordHeartbeat(t *testing.T, files *fake.FS) time.Time {
	t.Helper()
	var record leaseBody
	if err := json.Unmarshal(mustRead(t, files, leasePath), &record); err != nil {
		t.Fatalf("record: %v", err)
	}
	return record.Heartbeat
}

// waitFor yields until cond holds. It does not sleep; a broken heartbeat makes it fail after a bounded
// number of yields.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for range 100000 {
		if cond() {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("condition never held")
}
