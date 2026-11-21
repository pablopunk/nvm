package store

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestFiftyGoroutinesUpdateWhileReadersLoad(t *testing.T) {
	s, files, _ := newFixture(t)
	doc := settingsDocument(s)
	var writers, readers sync.WaitGroup
	done := make(chan struct{})
	for range 50 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for range 20 {
				if err := doc.Update(func(v *settingsDoc) { v.Count++ }); err != nil {
					t.Errorf("Update: %v", err)
					return
				}
			}
		}()
	}
	for range 4 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				if _, err := doc.Load(); err != nil {
					t.Errorf("Load: %v", err)
					return
				}
			}
		}()
	}
	writers.Wait()
	close(done)
	readers.Wait()

	got, err := doc.Load()
	if err != nil || got.Count != 1000 {
		t.Fatalf("Load = %+v, %v; want Count 1000 from 50x20 Updates", got, err)
	}
	if err := doc.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	version, _ := envelopeOf(t, mustRead(t, files, settingsPath))
	if version != 1 {
		t.Fatalf("file version = %d", version)
	}
}

func TestClosingTheStoreLeavesNoBackgroundGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()
	files := fake.NewFS()
	clock := fake.NewClock()
	s := openStoreOn(t, files, clock)
	for _, name := range []string{"one.json", "two.json", "three.json"} {
		doc := Doc(s, name, 1, freshSettingsDoc)
		if err := doc.Update(func(v *settingsDoc) { v.Count = 1 }); err != nil {
			t.Fatalf("Update %s: %v", name, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	waitForGoroutines(t, before)
}

func TestReleasingTheLeaseLeavesNoHeartbeatGoroutine(t *testing.T) {
	before := runtime.NumGoroutine()
	files := fake.NewFS()
	clock := fake.NewClock()
	lease, err := AcquireLease(files, clock, testDir, WithHeartbeat(time.Second, time.Minute))
	if err != nil {
		t.Fatalf("AcquireLease: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	waitForGoroutines(t, before)
}

// waitForGoroutines waits until the goroutine count is back at most at want. It yields the processor
// between checks instead of sleeping, so it finishes as soon as the goroutines have exited.
func waitForGoroutines(t *testing.T, want int) {
	t.Helper()
	for range 100000 {
		if runtime.NumGoroutine() <= want {
			return
		}
		runtime.Gosched()
	}
	t.Fatalf("goroutines = %d after shutdown, want at most %d", runtime.NumGoroutine(), want)
}
