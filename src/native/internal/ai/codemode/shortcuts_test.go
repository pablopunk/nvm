package codemode

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// fakeShortcutTable is the host's shortcut table: two commands, one of them bound, behind both the read and edit ports.
type fakeShortcutTable struct {
	mu    sync.Mutex
	bound map[string]string
	order []api.ShortcutRecord
}

func newFakeShortcutTable() *fakeShortcutTable {
	return &fakeShortcutTable{
		bound: map[string]string{"clipboard": "Command+Alt+V"},
		order: []api.ShortcutRecord{
			{ActionID: "clipboard", Title: "Clipboard History", Scope: "global", Source: "user"},
			{ActionID: "notes", Title: "Search Notes", Scope: "global", Source: "user"},
		},
	}
}

func (f *fakeShortcutTable) Bindable() []api.ShortcutRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	records := make([]api.ShortcutRecord, 0, len(f.order))
	for _, record := range f.order {
		record.Accelerator = f.bound[record.ActionID]
		records = append(records, record)
	}
	return records
}

func (f *fakeShortcutTable) List() []api.ShortcutRecord {
	var records []api.ShortcutRecord
	for _, record := range f.Bindable() {
		if record.Accelerator != "" {
			records = append(records, record)
		}
	}
	return records
}

func (f *fakeShortcutTable) Palette() api.ShortcutRecord {
	return api.ShortcutRecord{ActionID: "palette", Title: "Set Nevermind shortcut", Accelerator: "Command+Space"}
}

func (f *fakeShortcutTable) Aliases(string) map[string][]string { return nil }

// Plan prepares a change the way the host does: the command's title and current shortcut, and a refusal of a key
// another command took.
func (f *fakeShortcutTable) Plan(actionID, accelerator string) (protocol.ShortcutChange, error) {
	record := f.record(actionID)
	if record.ActionID == "" {
		return protocol.ShortcutChange{}, fmt.Errorf("no command %q: %w", actionID, api.ErrInvalidInput)
	}
	change := protocol.ShortcutChange{ActionID: actionID, Title: record.Title, Accelerator: accelerator, Current: record.Accelerator, Revision: f.revision()}
	for _, other := range f.Bindable() {
		if accelerator != "" && other.ActionID != actionID && other.Accelerator == accelerator {
			return change, fmt.Errorf("%s is already used by %s: %w", accelerator, other.Title, api.ErrInvalidInput)
		}
	}
	return change, nil
}

func (f *fakeShortcutTable) revision() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fmt.Sprint(f.bound)
}

func (f *fakeShortcutTable) Set(_ context.Context, actionID, accelerator, expect string) (api.ShortcutRecord, error) {
	if expect != "" && expect != f.revision() {
		return api.ShortcutRecord{}, fmt.Errorf("the shortcuts changed: %w", api.ErrInvalidInput)
	}
	f.mu.Lock()
	f.bound[actionID] = accelerator
	f.mu.Unlock()
	return f.record(actionID), nil
}

func (f *fakeShortcutTable) Remove(_ context.Context, actionID, expect string) error {
	if expect != "" && expect != f.revision() {
		return fmt.Errorf("the shortcuts changed: %w", api.ErrInvalidInput)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.bound, actionID)
	return nil
}

// bind changes a shortcut behind the program's back, as the user does in Settings while a question waits.
func (f *fakeShortcutTable) bind(actionID, accelerator string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bound[actionID] = accelerator
}

func (f *fakeShortcutTable) record(actionID string) api.ShortcutRecord {
	for _, record := range f.Bindable() {
		if record.ActionID == actionID {
			return record
		}
	}
	return api.ShortcutRecord{}
}

func (f *fakeShortcutTable) accelerator(actionID string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bound[actionID]
}

func withShortcuts(table *fakeShortcutTable) fixtureOption {
	return func(opts *Options) {
		opts.Shortcuts = table
		opts.ShortcutEditor = table
	}
}

func TestShortcutOperationsAreClassifiedByEffect(t *testing.T) {
	want := map[string]Class{
		protocol.OpList: ClassRead, protocol.OpPalette: ClassRead, protocol.OpBindable: ClassRead,
		protocol.OpSet: ClassSetting, protocol.OpRemove: ClassSetting,
	}
	rules := shortcutRules()
	if len(rules) != len(want) {
		t.Fatalf("the shortcuts table has %d rules; this test pins %d", len(rules), len(want))
	}
	for op, class := range want {
		rule, ok := rules[key{protocol.ServiceShortcuts, op}]
		if !ok || rule.class != class {
			t.Errorf("shortcuts.%s = %+v (listed %v); want %s", op, rule.class, ok, class)
		}
		if rule.label == "" || rule.describe == nil || rule.journal == nil {
			t.Errorf("shortcuts.%s lacks a label, a description or a journal line", op)
		}
	}
}

// A shortcut change asks with the command's title and current shortcut from the host, and runs once allowed.
func TestSettingAShortcutAsksWithHostNamesAndRunsWhenAllowed(t *testing.T) {
	table := newFakeShortcutTable()
	f := newFixture(t, withShortcuts(table))
	f.steps.decide = allow
	result := f.run(exampleBodies()["Set a shortcut (asks the user)"])
	if result.Failed {
		t.Fatalf("failed: %s", result.Text)
	}
	if got := table.accelerator("clipboard"); got != "Command+Shift+V" {
		t.Fatalf("clipboard shortcut = %q", got)
	}
	row := lastRow(t, f, "Changing a shortcut")
	if !row.asked || !strings.Contains(row.detail, "“Clipboard History”") || !strings.Contains(row.detail, "replacing Command+Alt+V") {
		t.Fatalf("row = %+v", row)
	}
	contains(t, result.Text, "set Command+Shift+V for “Clipboard History” (the user allowed it)")
}

func TestADeclinedShortcutChangeSavesNothing(t *testing.T) {
	table := newFakeShortcutTable()
	f := newFixture(t, withShortcuts(table))
	f.steps.decide = decline
	result := f.run(`return nil, ctx.Shortcuts().Remove("clipboard")`)
	if !result.Failed {
		t.Fatalf("a declined remove reported success: %s", result.Text)
	}
	if got := table.accelerator("clipboard"); got != "Command+Alt+V" {
		t.Fatalf("clipboard shortcut = %q after a decline", got)
	}
	contains(t, result.Text, "remove Command+Alt+V from “Clipboard History”: the user declined")
}

// A change the host refuses while preparing it fails with the host's reason, and the user is never asked.
func TestATakenAcceleratorFailsWithoutAsking(t *testing.T) {
	table := newFakeShortcutTable()
	f := newFixture(t, withShortcuts(table))
	f.steps.decide = allow
	result := f.run(`_, err := ctx.Shortcuts().Set("notes", "Command+Alt+V")
return nil, err`)
	if !result.Failed || !strings.Contains(result.Text, "already used") {
		t.Fatalf("result = %+v", result)
	}
	if got := table.accelerator("notes"); got != "" {
		t.Fatalf("notes shortcut = %q", got)
	}
	if row := lastRow(t, f, "Changing a shortcut"); row.asked {
		t.Fatalf("a change the host refused was put to the user: %+v", row)
	}
}

// The approval covers the table the user saw: a shortcut the user changed while the question waited is not
// overwritten by the approved change.
func TestAnApprovalDoesNotOverwriteAChangeMadeWhileItWaited(t *testing.T) {
	table := newFakeShortcutTable()
	f := newFixture(t, withShortcuts(table))
	f.steps.decide = func(*fakeRow, context.Context) (bool, error) {
		table.bind("clipboard", "Control+Alt+V")
		return true, nil
	}
	result := f.run(`_, err := ctx.Shortcuts().Set("clipboard", "Command+Shift+V")
return nil, err`)
	if !result.Failed || !strings.Contains(result.Text, "shortcuts changed") {
		t.Fatalf("result = %+v", result)
	}
	if got := table.accelerator("clipboard"); got != "Control+Alt+V" {
		t.Fatalf("clipboard shortcut = %q; the user's newer choice was overwritten", got)
	}
}

func TestReadingShortcutsDoesNotAsk(t *testing.T) {
	f := newFixture(t, withShortcuts(newFakeShortcutTable()))
	result := f.run(`return len(ctx.Shortcuts().List()), nil`)
	if result.Failed || !strings.Contains(result.Text, "Result: 1") {
		t.Fatalf("result = %+v", result)
	}
	for _, row := range f.steps.snapshot() {
		if row.asked {
			t.Fatalf("a read asked: %+v", row)
		}
	}
}

func lastRow(t *testing.T, f *fixture, label string) fakeRow {
	t.Helper()
	rows := f.steps.snapshot()
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].label == label {
			return rows[i]
		}
	}
	t.Fatalf("no %q row in %+v", label, rows)
	return fakeRow{}
}
