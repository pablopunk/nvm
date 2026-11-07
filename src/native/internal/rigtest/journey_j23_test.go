//go:build rig

package rigtest

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestJourneyJ23DestructiveCommandAsksFirstAndRunsOnConfirm is the J23 smoke. Empty Trash is destructive: it opens a
// confirmation, Cancel leaves the system alone, and the confirmed command runs once. Restart runs at once, as 1.x runs
// Restart Computer, with no confirmation. The app's System port is a recorder in this build, so the calls are written to
// the test's log and nothing restarts. Lock, sleep, log out and shut down are not scripted.
func TestJourneyJ23DestructiveCommandAsksFirstAndRunsOnConfirm(t *testing.T) {
	app := Launch(t, Options{})
	must(t, app.SummonPalette())

	must(t, app.Type("empty trash"))
	_, err := app.WaitFor("Empty Trash", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Empty Trash?", elementWait)
	must(t, err)
	shoot(t, app, "j23-1-confirmation")
	if calls := systemCalls(t, app); len(calls) != 0 {
		t.Fatalf("Empty Trash ran before confirmation: %v", calls)
	}

	must(t, app.Key("Down"))
	must(t, app.Key("Return"))
	must(t, app.Type("empty trash"))
	_, err = app.WaitFor("Empty Trash", elementWait)
	must(t, err)
	if calls := systemCalls(t, app); len(calls) != 0 {
		t.Fatalf("Cancel ran a system command: %v", calls)
	}

	must(t, app.Key("Return"))
	must(t, app.Key("Return"))
	waitSystemCalls(t, app, []string{"EmptyTrash"})
	shoot(t, app, "j23-2-confirmed")

	must(t, app.SummonPalette())
	must(t, app.Type("restart"))
	_, err = app.WaitFor("Restart Computer", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	waitSystemCalls(t, app, []string{"EmptyTrash", "Restart"})
	shoot(t, app, "j23-3-restart-runs-at-once")
}

// systemCalls reads the system commands the app recorded in this launch.
func systemCalls(t *testing.T, app *App) []string {
	t.Helper()
	data, err := os.ReadFile(app.systemLog())
	if os.IsNotExist(err) {
		return nil
	}
	must(t, err)
	return strings.Fields(string(data))
}

// waitSystemCalls waits until the recorded calls equal want, which the confirmed command writes asynchronously.
func waitSystemCalls(t *testing.T, app *App, want []string) {
	t.Helper()
	deadline := time.Now().Add(elementWait)
	for {
		if strings.Join(systemCalls(t, app), " ") == strings.Join(want, " ") {
			return
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("system calls = %v, want %v", systemCalls(t, app), want)
}
