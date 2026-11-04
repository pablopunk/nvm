package audiomute

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// memoryJournal holds one entry, as the app's journal file does.
type memoryJournal struct {
	entry     *bool
	saveErr   error
	clearedAt int
}

func (j *memoryJournal) Save(wasMuted bool) error {
	if j.saveErr != nil {
		return j.saveErr
	}
	j.entry = &wasMuted
	return nil
}

func (j *memoryJournal) Load() (bool, bool, error) {
	if j.entry == nil {
		return false, false, nil
	}
	return *j.entry, true, nil
}

func (j *memoryJournal) Clear() error {
	j.entry = nil
	j.clearedAt++
	return nil
}

func TestBeginMutesAndRestoreGivesBackThePreviousState(t *testing.T) {
	output := fake.NewOutputMute()
	journal := &memoryJournal{}
	lease, err := New(output, journal).Begin()
	if err != nil || lease == nil {
		t.Fatalf("Begin = %v, %v; want a lease", lease, err)
	}
	if muted, _ := output.Muted(); !muted {
		t.Fatalf("output not muted after Begin")
	}
	if err := lease.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if muted, _ := output.Muted(); muted {
		t.Fatalf("output still muted after Restore")
	}
	if journal.entry != nil {
		t.Fatalf("journal kept an entry after a clean restore")
	}
}

func TestAnOutputThatWasAlreadyMutedStaysMuted(t *testing.T) {
	output := fake.NewOutputMute()
	output.SetState(true)
	lease, _ := New(output, &memoryJournal{}).Begin()
	_ = lease.Restore()
	if muted, _ := output.Muted(); !muted {
		t.Fatalf("a mute the user had set was lifted by the restore")
	}
}

func TestRestoreIsIdempotentAndSafeOnANilLease(t *testing.T) {
	output := fake.NewOutputMute()
	lease, _ := New(output, &memoryJournal{}).Begin()
	_ = lease.Restore()
	_ = lease.Restore()
	if changes := output.Changes(); len(changes) != 2 {
		t.Fatalf("changes = %v, want one mute and one restore", changes)
	}
	var none *Lease
	if err := none.Restore(); err != nil {
		t.Fatalf("Restore of a nil lease = %v", err)
	}
}

func TestAnOSThatCannotMuteGetsNoLeaseAndNoError(t *testing.T) {
	output := fake.NewOutputMute()
	output.FailNext("output.muted", platform.ErrUnsupported)
	lease, err := New(output, &memoryJournal{}).Begin()
	if lease != nil || err != nil {
		t.Fatalf("Begin = %v, %v; want no lease and no error", lease, err)
	}
}

func TestAJournalThatCannotBeWrittenStopsTheMute(t *testing.T) {
	output := fake.NewOutputMute()
	journal := &memoryJournal{saveErr: errors.New("read only")}
	if _, err := New(output, journal).Begin(); err == nil {
		t.Fatalf("Begin with a journal that cannot be written succeeded")
	}
	if len(output.Changes()) != 0 {
		t.Fatalf("the output was muted without a journal entry: %v", output.Changes())
	}
}

func TestACrashLeavesAJournalThatTheNextStartGivesBack(t *testing.T) {
	output := fake.NewOutputMute()
	journal := &memoryJournal{}
	if _, err := New(output, journal).Begin(); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	// The app stops here; a new muter starts with the same journal and a muted output.
	if err := New(output, journal).RecoverAfterCrash(); err != nil {
		t.Fatalf("RecoverAfterCrash: %v", err)
	}
	if muted, _ := output.Muted(); muted {
		t.Fatalf("output still muted after recovery")
	}
	if err := New(output, journal).RecoverAfterCrash(); err != nil || len(output.Changes()) != 2 {
		t.Fatalf("a second recovery changed the output again: %v", output.Changes())
	}
}
