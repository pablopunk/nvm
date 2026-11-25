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

func (f *fakeShortcutTable) Set(_ context.Context, actionID, accelerator string) (api.ShortcutRecord, error) {
	f.mu.Lock()
	for id, taken := range f.bound {
		if taken == accelerator && id != actionID {
			f.mu.Unlock()
			return api.ShortcutRecord{}, fmt.Errorf("%s is already used: %w", accelerator, api.ErrInvalidInput)
		}
	}
	f.bound[actionID] = accelerator
	f.mu.Unlock()
	return f.record(actionID), nil
}

func (f *fakeShortcutTable) Remove(_ context.Context, actionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.bound, actionID)
	return nil
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

// A refused accelerator reaches the program as the host's reason, and the journal says it failed.
func TestATakenAcceleratorFailsWithTheHostReason(t *testing.T) {
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
