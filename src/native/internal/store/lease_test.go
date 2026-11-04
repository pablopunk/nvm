package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// leasePath is the lock record inside the test store directory.
var leasePath = filepath.Join(testDir, leaseFileName)

// quietLease configures a lease that does not renew itself, so tests control every heartbeat.
func quietLease() LeaseOption {
	return WithHeartbeat(0, 30*time.Second)
}

// mustAcquire claims the store directory or fails the test.
func mustAcquire(t *testing.T, files *fake.FS, clock *fake.Clock, opts ...LeaseOption) *Lease {
	t.Helper()
	lease, err := AcquireLease(files, clock, testDir, opts...)
	if err != nil {
		t.Fatalf("AcquireLease: %v", err)
	}
	return lease
}

// seedRecord writes a lock record as another instance would have left it.
func seedRecord(t *testing.T, files *fake.FS, body leaseBody) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	files.Seed(leasePath, data)
}

func TestAcquireWritesRecordAndReleaseRemovesIt(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	lease := mustAcquire(t, files, clock, quietLease())
	var record leaseBody
	if err := json.Unmarshal(mustRead(t, files, leasePath), &record); err != nil {
		t.Fatalf("record: %v", err)
	}
	if record.PID != os.Getpid() || record.Token == "" {
		t.Fatalf("record = %+v, want this process and a token", record)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, err := files.ReadFile(leasePath); err == nil {
		t.Fatal("Release left the record behind")
	}
}

func TestSecondInstanceIsRefusedWithHolderDetails(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	first := mustAcquire(t, files, clock, quietLease())
	defer func() { _ = first.Release() }()

	_, err := AcquireLease(files, clock, testDir, quietLease())
	if !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("second AcquireLease = %v, want ErrLeaseHeld", err)
	}
	var held *LeaseHeldError
	if !errors.As(err, &held) || held.Holder.PID != os.Getpid() {
		t.Fatalf("holder details = %+v", held)
	}
}

func TestConcurrentClaimsHaveExactlyOneWinner(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	var mu sync.Mutex
	var winners []*Lease
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lease, err := AcquireLease(files, clock, testDir, quietLease())
			if err != nil {
				if !errors.Is(err, ErrLeaseHeld) {
					t.Errorf("AcquireLease: %v", err)
				}
				return
			}
			mu.Lock()
			winners = append(winners, lease)
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(winners) != 1 {
		t.Fatalf("%d instances acquired the lease, want exactly one", len(winners))
	}
	if err := winners[0].Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestStaleHeartbeatIsTakenOverAndTheOldHolderIsFenced(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	old := mustAcquire(t, files, clock, quietLease())
	clock.Advance(31 * time.Second)

	successor := mustAcquire(t, files, clock, quietLease())
	defer func() { _ = successor.Release() }()

	if err := old.Verify(); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("old holder Verify = %v, want ErrLeaseLost", err)
	}
	if err := old.Release(); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("old holder Release = %v, want ErrLeaseLost and the record kept", err)
	}
	if _, err := files.ReadFile(leasePath); err != nil {
		t.Fatalf("the successor's record was removed: %v", err)
	}
}

func TestLostLeaseStopsWritesBeforeAnythingIsOverwritten(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	old := mustAcquire(t, files, clock, quietLease())
	s, err := Open(files, clock, testDir, WithDebounce(time.Hour), WithLease(old))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	doc := settingsDocument(s)
	mustUpdate(t, doc, func(v *settingsDoc) { v.Title = "from the old holder" })

	clock.Advance(31 * time.Second)
	successor := mustAcquire(t, files, clock, quietLease())
	defer func() { _ = successor.Release() }()

	if err := doc.Flush(); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("Flush after takeover = %v, want ErrLeaseLost", err)
	}
	if err := doc.Update(func(v *settingsDoc) { v.Count = 9 }); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("Update after takeover = %v, want ErrLeaseLost", err)
	}
	if _, err := files.Stat(settingsPath); err == nil {
		t.Fatal("the fenced holder wrote the settings file")
	}
}

func TestDeadProcessIsTakenOverAtOnce(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	now := clock.Now()
	seedRecord(t, files, leaseBody{PID: 4242, Token: "dead", Started: now, Heartbeat: now})
	living := fake.NewProcesses(platform.Process{PID: 1, Name: "init"})

	lease, err := AcquireLease(files, clock, testDir, WithProcesses(living), quietLease())
	if err != nil {
		t.Fatalf("AcquireLease over a dead process: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestLivingProcessWithFreshHeartbeatKeepsTheLease(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	now := clock.Now()
	seedRecord(t, files, leaseBody{PID: 4242, Token: "live", Started: now, Heartbeat: now})
	living := fake.NewProcesses(platform.Process{PID: 4242, Name: "other"})

	_, err := AcquireLease(files, clock, testDir, WithProcesses(living), quietLease())
	if !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("AcquireLease = %v, want ErrLeaseHeld", err)
	}
}

// A living holder whose heartbeat has stopped is taken over: a suspended or wedged writer must not block
// the store forever. Verify before each write fences it if it wakes up.
func TestSilentLivingHolderIsTakenOverOnceItsHeartbeatExpires(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	now := clock.Now()
	seedRecord(t, files, leaseBody{PID: 4242, Token: "silent", Started: now, Heartbeat: now})
	clock.Advance(time.Minute)
	living := fake.NewProcesses(platform.Process{PID: 4242, Name: "other"})

	lease, err := AcquireLease(files, clock, testDir, WithProcesses(living), quietLease())
	if err != nil {
		t.Fatalf("AcquireLease over a silent holder: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestWithoutProcessListingOnlyTheHeartbeatDecides(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	now := clock.Now()
	seedRecord(t, files, leaseBody{PID: 4242, Token: "fresh", Started: now, Heartbeat: now})

	_, err := AcquireLease(files, clock, testDir, quietLease())
	if !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("AcquireLease with a fresh heartbeat = %v, want ErrLeaseHeld", err)
	}
}

func TestUnreadableRecordWithOldFileIsTakenOver(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	files.Seed(leasePath, []byte("{half a wri"))
	lease := mustAcquire(t, files, clock, quietLease())
	if err := lease.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestRenewKeepsTheLeaseFresh(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	lease := mustAcquire(t, files, clock, quietLease())
	for range 5 {
		clock.Advance(10 * time.Second)
		if err := lease.Renew(); err != nil {
			t.Fatalf("Renew: %v", err)
		}
	}
	other, err := AcquireLease(files, clock, testDir, quietLease())
	if !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("takeover of a renewed lease = %v, want ErrLeaseHeld", err)
	}
	if other != nil {
		_ = other.Release()
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestClosingTheStoreReleasesItsLease(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	lease := mustAcquire(t, files, clock, quietLease())
	s, err := Open(files, clock, testDir, WithLease(lease))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	next, err := AcquireLease(files, clock, testDir, quietLease())
	if err != nil {
		t.Fatalf("lease not released by Close: %v", err)
	}
	if err := next.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestReleaseTwiceIsHarmless(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	lease := mustAcquire(t, files, clock, quietLease())
	if err := lease.Release(); err != nil {
		t.Fatalf("first Release: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("second Release: %v", err)
	}
	if err := lease.Renew(); !errors.Is(err, ErrLeaseReleased) {
		t.Fatalf("Renew after Release = %v, want ErrLeaseReleased", err)
	}
}

func TestLeaseTimeoutMustBePositive(t *testing.T) {
	_, err := AcquireLease(fake.NewFS(), fake.NewClock(), testDir, WithHeartbeat(0, 0))
	if err == nil {
		t.Fatal("a zero timeout was accepted")
	}
}
