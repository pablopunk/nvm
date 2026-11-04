package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// TestRealFileSystemRoundTripsAndKeepsPrivateModes runs the store on the operating system's file system: the
// fake covers the logic, this covers real rename, fsync and permission bits.
func TestRealFileSystemRoundTripsAndKeepsPrivateModes(t *testing.T) {
	ports := real.NewPorts()
	dir := t.TempDir()
	s, err := Open(ports.FS, ports.Clock, dir, WithDebounce(time.Hour))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	doc := Doc(s, "settings.json", 1, freshSettingsDoc)
	mustUpdate(t, doc, func(v *settingsDoc) { v.Title = "on disk" })
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := Open(ports.FS, ports.Clock, dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = reopened.Close() }()
	got, err := Doc(reopened, "settings.json", 1, freshSettingsDoc).Load()
	if err != nil || got.Title != "on disk" {
		t.Fatalf("Load after reopen = %+v, %v", got, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != "settings.json" {
			t.Fatalf("unexpected file left in the store directory: %s", entry.Name())
		}
	}
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(filepath.Join(dir, "settings.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("stored file mode = %v, %v; want 0600", info.Mode().Perm(), err)
	}
}

// TestRealLeaseRefusesALiveHolderAndTakesOverADeadOne uses the real process table. It is skipped where the OS
// cannot list processes, which is the documented heartbeat-only fallback.
func TestRealLeaseRefusesALiveHolderAndTakesOverADeadOne(t *testing.T) {
	ports := real.NewPorts()
	if !ports.Capabilities.Supported(platform.CapabilityProcessList) {
		t.Skip("this OS cannot list processes; the heartbeat rule covers it")
	}
	dir := t.TempDir()
	first, err := AcquireLease(ports.FS, ports.Clock, dir, WithProcesses(ports.Processes), WithHeartbeat(0, time.Minute))
	if err != nil {
		t.Fatalf("first AcquireLease: %v", err)
	}
	if _, err := AcquireLease(ports.FS, ports.Clock, dir, WithProcesses(ports.Processes), WithHeartbeat(0, time.Minute)); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("second AcquireLease = %v, want ErrLeaseHeld", err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}

	now := ports.Clock.Now()
	deadRecord := leaseBody{PID: 4194303, Token: "dead", Started: now, Heartbeat: now}
	writeRealRecord(t, dir, deadRecord)
	successor, err := AcquireLease(ports.FS, ports.Clock, dir, WithProcesses(ports.Processes), WithHeartbeat(0, time.Minute))
	if err != nil {
		t.Fatalf("AcquireLease over a dead process: %v", err)
	}
	if err := successor.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

// TestRealLeaseKeepsAFreshUnreadableRecord checks the real file's modification time: a torn record that was
// just written may belong to an instance that is still starting, so it is not taken over.
func TestRealLeaseKeepsAFreshUnreadableRecord(t *testing.T) {
	ports := real.NewPorts()
	dir := t.TempDir()
	path := filepath.Join(dir, leaseFileName)
	if err := os.WriteFile(path, []byte("{torn"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := AcquireLease(ports.FS, ports.Clock, dir, WithHeartbeat(0, time.Minute))
	if !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("AcquireLease over a fresh torn record = %v, want ErrLeaseHeld", err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	lease, err := AcquireLease(ports.FS, ports.Clock, dir, WithHeartbeat(0, time.Minute))
	if err != nil {
		t.Fatalf("AcquireLease over an old torn record: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

// writeRealRecord writes a lock record to a real directory.
func writeRealRecord(t *testing.T, dir string, body leaseBody) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, leaseFileName), data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
