package windowmgmt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// memoryLog is a logger that keeps its entries in memory, so a test reads back what the engine wrote.
func memoryLog(t *testing.T) *logx.Logger {
	t.Helper()
	log, err := logx.New(nil, fake.NewClock(), "", logx.Options{Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	return log
}

// failedCommandEntries returns the warnings a failed window command wrote.
func failedCommandEntries(log *logx.Logger) []logx.Entry {
	var entries []logx.Entry
	for _, entry := range log.Recent(0) {
		if entry.Message == "window.command.failed" {
			entries = append(entries, entry)
		}
	}
	return entries
}

func TestDeniedAccessNamesAccessibilityAndLogsTheRefusal(t *testing.T) {
	h := newHarness(t)
	log := memoryLog(t)
	h.service.log = log
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	h.windows.SetAccess(platform.WindowAccessDenied)
	h.windows.Focus(key)

	_, err := h.service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), *h.shortcutTarget())
	requireFailure(t, err, ReasonDenied)
	if err.Error() != messageDenied {
		t.Fatalf("palette copy = %q, want the Accessibility guidance", err.Error())
	}
	entries := failedCommandEntries(log)
	if len(entries) != 1 || entries[0].Fields["reason"] != string(ReasonDenied) || entries[0].Fields["op"] != "apply" {
		t.Fatalf("log entries = %+v, want one denied apply warning", entries)
	}
}

func TestHelperFailureShowsTheHelperCopyAndLogsItsCause(t *testing.T) {
	h := newHarness(t)
	log := memoryLog(t)
	h.service.log = log
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	h.windows.Focus(key)
	h.windows.FailNext("Access", &platform.WindowError{Code: platform.WindowErrorFailed,
		Message: "fork/exec macos-selected-text: no such file or directory"})

	_, err := h.service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), *h.shortcutTarget())
	requireFailure(t, err, ReasonUnapplied)
	if err.Error() != messageHelperFailed {
		t.Fatalf("palette copy = %q, want the helper copy", err.Error())
	}
	entries := failedCommandEntries(log)
	if len(entries) != 1 || fmt.Sprint(entries[0].Fields["cause"]) != "fork/exec macos-selected-text: no such file or directory" {
		t.Fatalf("log entries = %+v, want the helper's cause", entries)
	}
}

func TestFrontmostFailureReadsAsNoTargetAndIsLogged(t *testing.T) {
	h := newHarness(t)
	log := memoryLog(t)
	h.service.log = log
	h.windows.FailNext("Frontmost", errors.New("lsappinfo could not run"))

	if _, ok := h.service.FrontmostTarget(context.Background()); ok {
		t.Fatal("a front app was named although the window system failed")
	}
	entries := failedCommandEntries(log)
	if len(entries) != 1 || entries[0].Fields["op"] != "frontmost" {
		t.Fatalf("log entries = %+v, want one frontmost warning", entries)
	}
}

func TestSuccessfulCommandWritesNoWarning(t *testing.T) {
	h := newHarness(t)
	log := memoryLog(t)
	h.service.log = log
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	h.windows.Focus(key)

	if _, err := h.service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), *h.shortcutTarget()); err != nil {
		t.Fatal(err)
	}
	if entries := failedCommandEntries(log); len(entries) != 0 {
		t.Fatalf("a command that reached its window logged %+v", entries)
	}
}
