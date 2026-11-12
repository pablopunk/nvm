package shortcuts

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// fakeSystemShortcuts answers with what the test sets, and counts its reads.
type fakeSystemShortcuts struct {
	mu    sync.Mutex
	list  []platform.SystemShortcut
	err   error
	reads int
}

func (f *fakeSystemShortcuts) Enabled(context.Context) ([]platform.SystemShortcut, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	return f.list, f.err
}

func (f *fakeSystemShortcuts) set(list []platform.SystemShortcut, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.list, f.err = list, err
}

var screenshotShortcut = platform.SystemShortcut{Name: "Save picture of selected area as a file", Key: "4", Command: true, Shift: true}

func systemFixture(t *testing.T, source platform.SystemShortcuts) (*managerFixture, *fake.Clock) {
	t.Helper()
	f := &managerFixture{hotkeys: newRecordingHotkeys(), doc: NewMemoryDocument(DefaultDocument("Alt+Shift+Space"))}
	clock := fake.NewClock()
	f.manager = NewManager(Setup{
		Doc: f.doc, Hotkeys: f.hotkeys, Clock: clock, GOOS: "darwin", Commands: testCommands,
		Run: func(string) {}, OnPalette: func() {}, System: source,
	})
	return f, clock
}

// settle waits for the background read to land.
func settle(t *testing.T, m *Manager, accelerator string, want bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if (m.system.Owner(accelerator) != "") == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("system owner of %s never became %v", accelerator, want)
}

func TestRecordingRefusesAShortcutTheOSHasTurnedOnAndNamesIt(t *testing.T) {
	source := &fakeSystemShortcuts{list: []platform.SystemShortcut{screenshotShortcut}}
	f, _ := systemFixture(t, source)
	f.manager.Start()
	settle(t, f.manager, "Command+Shift+4", true)

	err := f.manager.SetAction("command:notes", "Shift+Command+4")
	if err == nil || !strings.Contains(err.Error(), "Save picture of selected area as a file") {
		t.Fatalf("SetAction error = %v, want a refusal naming the system shortcut", err)
	}
	if doc, _ := f.doc.Load(); doc.Actions["command:notes"] != "" {
		t.Fatal("the refused shortcut was saved")
	}
}

func TestRecordingAllowsAShortcutTheOSDoesNotListOrHasTurnedOff(t *testing.T) {
	source := &fakeSystemShortcuts{list: []platform.SystemShortcut{screenshotShortcut}}
	f, _ := systemFixture(t, source)
	f.manager.Start()
	settle(t, f.manager, "Command+Shift+4", true)

	if err := f.manager.SetAction("command:notes", "Command+Tab"); err != nil {
		t.Fatalf("Command+Tab is not in the symbolic hotkeys but was refused: %v", err)
	}
	source.set(nil, nil) // the user turned it off in System Settings
	f.clockAdvance(t)
	settle(t, f.manager, "Command+Shift+4", false)
	if err := f.manager.SetAction("command:notes", "Shift+Command+4"); err != nil {
		t.Fatalf("a shortcut the OS no longer uses was refused: %v", err)
	}
}

func (f *managerFixture) clockAdvance(t *testing.T) {
	t.Helper()
	f.manager.system.clock.(*fake.Clock).Advance(systemFreshFor + time.Second)
}

func TestRecordingAllowsEverythingWhenTheSystemShortcutsCannotBeRead(t *testing.T) {
	source := &fakeSystemShortcuts{err: errors.New("defaults failed")}
	f, _ := systemFixture(t, source)
	f.manager.Start()
	time.Sleep(20 * time.Millisecond)
	if err := f.manager.SetAction("command:notes", "Command+Shift+4"); err != nil {
		t.Fatalf("a failed read blocked the shortcut: %v", err)
	}
}

func TestRecordingWithoutASystemSourceRefusesNothingNew(t *testing.T) {
	f, _ := systemFixture(t, nil)
	f.manager.Start()
	if err := f.manager.SetAction("command:notes", "Command+Shift+4"); err != nil {
		t.Fatalf("SetAction() = %v", err)
	}
}
