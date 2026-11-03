package store

import (
	"bytes"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// settingsTemp is the temporary file a settings write goes through.
const settingsTemp = testDir + "/.settings.json.tmp"

// errDisk is the injected I/O failure.
var errDisk = errors.New("disk I/O error")

// crashSignal is the panic value that simulates the process dying in the middle of a write.
type crashSignal struct{}

// commitSettings writes value through a store and closes it, so later steps start from a committed file. It
// returns the committed bytes.
func commitSettings(t *testing.T, files *fake.FS, clock *fake.Clock, value settingsDoc) []byte {
	t.Helper()
	s := openStoreOn(t, files, clock)
	doc := settingsDocument(s)
	if err := doc.Update(func(v *settingsDoc) { *v = value }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return mustRead(t, files, settingsPath)
}

// openStoreOn opens a store without registering a cleanup, so a test can simulate a dead process by simply
// not closing it.
func openStoreOn(t *testing.T, files *fake.FS, clock *fake.Clock) *Store {
	t.Helper()
	s, err := Open(files, clock, testDir, WithDebounce(time.Hour))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

// underStoreDir reports whether a file system write targets the store directory.
func underStoreDir(name string) bool {
	return strings.HasPrefix(name, testDir+"/")
}

// flushSurvivingCrash runs Flush and reports whether it died with the crash signal. Any other panic is
// re-raised.
func flushSurvivingCrash(doc *Collection[settingsDoc]) (crashed bool, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if _, isCrash := recovered.(crashSignal); !isCrash {
				panic(recovered)
			}
			crashed = true
		}
	}()
	return false, doc.Flush()
}

// writeFailure describes one way a save can be interrupted.
type writeFailure struct {
	name    string
	arm     func(files *fake.FS)
	crashes bool
}

// writeFailures lists the interruptions the crash tests cover. Each one must leave the committed file whole.
func writeFailures() []writeFailure {
	return []writeFailure{
		{
			name: "temporary write fails",
			arm:  func(files *fake.FS) { files.FailNext(fake.OpWriteFile, errDisk) },
		},
		{
			name: "fsync of the temporary file fails",
			arm:  func(files *fake.FS) { files.FailNext(fake.OpSync, errDisk) },
		},
		{
			name: "rename fails",
			arm:  func(files *fake.FS) { files.FailNext(fake.OpRename, errDisk) },
		},
		{
			name: "hook fails after the temporary write",
			arm: func(files *fake.FS) {
				files.AfterWrite(func(name string) error {
					if name == settingsTemp {
						return errDisk
					}
					return nil
				})
			},
		},
		{
			name:    "process dies after the temporary write",
			crashes: true,
			arm: func(files *fake.FS) {
				files.AfterWrite(func(name string) error {
					if underStoreDir(name) {
						panic(crashSignal{})
					}
					return nil
				})
			},
		},
		{
			name:    "process dies while the temporary file is torn",
			crashes: true,
			arm: func(files *fake.FS) {
				files.AfterWrite(func(name string) error {
					if underStoreDir(name) {
						files.Seed(settingsTemp, []byte(`{"schemaVersion":1,"da`))
						panic(crashSignal{})
					}
					return nil
				})
			},
		},
	}
}

func TestInterruptedWriteLeavesCommittedFileWhole(t *testing.T) {
	for _, tc := range writeFailures() {
		t.Run(tc.name, func(t *testing.T) {
			files := fake.NewFS()
			clock := fake.NewClock()
			committed := commitSettings(t, files, clock, settingsDoc{Title: "old"})

			s := openStoreOn(t, files, clock)
			doc := settingsDocument(s)
			if err := doc.Update(func(v *settingsDoc) { v.Title = "new" }); err != nil {
				t.Fatalf("Update: %v", err)
			}
			tc.arm(files)
			crashed, err := flushSurvivingCrash(doc)
			files.AfterWrite(nil)
			if crashed != tc.crashes {
				t.Fatalf("crashed = %v, want %v", crashed, tc.crashes)
			}
			if !tc.crashes && err == nil {
				t.Fatal("the injected failure did not fail the flush")
			}
			if got := mustRead(t, files, settingsPath); !bytes.Equal(got, committed) {
				t.Fatalf("interrupted write changed the committed file:\n%s", got)
			}

			restarted := openStoreOn(t, files, clock)
			got, err := settingsDocument(restarted).Load()
			if err != nil || got.Title != "old" {
				t.Fatalf("after restart Load = %+v, %v; want the committed state", got, err)
			}
			if _, err := files.Stat(settingsTemp); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("leftover temporary file survived the next load: %v", err)
			}
			if !tc.crashes {
				assertRetrySucceeds(t, doc, files)
			}
		})
	}
}

// assertRetrySucceeds checks that a change whose write failed is still pending and lands on the next flush.
func assertRetrySucceeds(t *testing.T, doc *Collection[settingsDoc], files *fake.FS) {
	t.Helper()
	if err := doc.Flush(); err != nil {
		t.Fatalf("retry Flush: %v", err)
	}
	version, data := envelopeOf(t, mustRead(t, files, settingsPath))
	if version != 1 || !bytes.Contains(data, []byte(`"new"`)) {
		t.Fatalf("retry did not write the pending change: %s", data)
	}
}

func TestFailedSaveRaisesNoticeAndKeepsChangePending(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	commitSettings(t, files, clock, settingsDoc{Title: "old"})
	s := openStoreOn(t, files, clock)
	doc := settingsDocument(s)
	if err := doc.Update(func(v *settingsDoc) { v.Title = "new" }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	files.FailNext(fake.OpRename, errDisk)
	if err := doc.Flush(); !errors.Is(err, errDisk) {
		t.Fatalf("Flush = %v, want the injected error", err)
	}
	notice, ok := findNotice(drainNotices(s.Notices()), SaveFailed)
	if !ok || !strings.Contains(notice.Reason, "disk I/O error") {
		t.Fatalf("SaveFailed notice = %+v, %v", notice, ok)
	}
	assertRetrySucceeds(t, doc, files)
}
