package services

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	appfeature "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

const notesApp = "/Applications/Notes.app"

// lifecycleRecorder is the app lifecycle port a test inspects: which apps it was asked about and the answers it gives.
type lifecycleRecorder struct {
	running []appfeature.App
	asked   []appfeature.App
	quit    []string
	forced  []string
	hidden  []string
	outcome appfeature.QuitOutcome
	err     error
}

// IndexedApps lists the one app the test shell lists, as the index would.
func (r *lifecycleRecorder) IndexedApps() []appfeature.App {
	return []appfeature.App{{Name: "Notes", ID: "com.example.notes", Path: notesApp}}
}

func (r *lifecycleRecorder) TrashFolder() string { return "" }

func (r *lifecycleRecorder) DiscoverUninstall(string) appfeature.Discovery {
	return appfeature.Discovery{}
}

func (r *lifecycleRecorder) TrashUninstall(appfeature.UninstallSnapshot, map[string]bool) appfeature.TrashResult {
	return appfeature.TrashResult{}
}

func (r *lifecycleRecorder) RunningApps(list []appfeature.App) ([]appfeature.App, error) {
	r.asked = append(r.asked, list...)
	if r.err != nil {
		return nil, r.err
	}
	var out []appfeature.App
	for _, app := range list {
		for _, running := range r.running {
			if running.Path == app.Path {
				out = append(out, app)
			}
		}
	}
	return out, nil
}

func (r *lifecycleRecorder) QuitApp(app appfeature.App) (appfeature.QuitOutcome, error) {
	r.quit = append(r.quit, app.Path)
	return r.outcome, r.err
}

func (r *lifecycleRecorder) ForceQuitApp(app appfeature.App) (appfeature.QuitOutcome, error) {
	r.forced = append(r.forced, app.Path)
	return r.outcome, r.err
}

func (r *lifecycleRecorder) QuitAllApps() ([]appfeature.App, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.running, nil
}

func (r *lifecycleRecorder) HideApp(app appfeature.App) error {
	r.hidden = append(r.hidden, app.Path)
	return r.err
}

func lifecycleTestHandler(t *testing.T, port AppLifecycle, capabilities ...platform.Capability) (*Handler, *[]string, func(string) bool) {
	t.Helper()
	r := newRig(t)
	shell := fake.NewShell()
	shell.SetApps([]platform.App{{Name: "Notes", ID: "com.example.notes", Path: notesApp}}, nil)
	var notices []string
	h := New(Deps{
		Files:        real.NewPorts().FS,
		Clock:        r.clock,
		DataRoot:     r.root,
		Logs:         r.logs,
		Shell:        shell,
		Capabilities: platform.NewCapabilities(capabilities...),
		Apps:         port,
		Notify: func(title, message string) {
			notices = append(notices, title+": "+message)
		},
	}, manifestFor("acme.apps", api.CapabilityDesktopApps, api.CapabilitySystem))
	t.Cleanup(func() { _ = h.Close() })
	return h, &notices, func(message string) bool { return hasWarning(r.logs, message) }
}

// TestRunningAnswersOnlyForIndexedAppsThatRun: a path the index does not list is never running, and the answer keeps
// the request order.
func TestRunningAnswersOnlyForIndexedAppsThatRun(t *testing.T) {
	port := &lifecycleRecorder{running: []appfeature.App{{Name: "Notes", Path: notesApp}}}
	h, _, _ := lifecycleTestHandler(t, port, platform.CapabilityAppRunning)
	var running []string
	mustDecode(t, mustOK(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpRunning, protocol.PathsArgs{Paths: []string{"/Applications/Ghost.app", notesApp}})), &running)
	if len(running) != 1 || running[0] != notesApp {
		t.Fatalf("running = %v; want Notes only", running)
	}
	if len(port.asked) != 1 || port.asked[0].Path != notesApp {
		t.Fatalf("the port was asked about %+v; want only the indexed app", port.asked)
	}
}

// TestRunningRefusesMoreThanTheCap keeps one palette list from asking about the whole index at once.
func TestRunningRefusesMoreThanTheCap(t *testing.T) {
	h, _, _ := lifecycleTestHandler(t, &lifecycleRecorder{}, platform.CapabilityAppRunning)
	paths := make([]string, api.MaxRunningPaths+1)
	requireCode(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpRunning, protocol.PathsArgs{Paths: paths}), api.CodeInvalid, "at most")
}

// TestLifecycleIsRefusedWithoutItsCapability: an OS that cannot report running apps, or hide them, answers in words
// the palette shows, and the port is never asked.
func TestLifecycleIsRefusedWithoutItsCapability(t *testing.T) {
	port := &lifecycleRecorder{}
	h, _, _ := lifecycleTestHandler(t, port)
	reply := serve(t, h, protocol.ServiceDesktopApps, protocol.OpRunning, protocol.PathsArgs{Paths: []string{notesApp}})
	if got := failureCode(t, reply); got.Code != api.CodeUnavailable || !strings.Contains(got.Message, "not available on this system") {
		t.Fatalf("running without the capability = %+v; want an unavailable answer", got)
	}
	requireCode(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpHideApp, protocol.PathArgs{Path: notesApp}), api.CodeUnavailable, "not available")
	if len(port.asked) != 0 || len(port.hidden) != 0 {
		t.Fatalf("refused calls reached the port: asked %+v hidden %v", port.asked, port.hidden)
	}
}

// TestQuitAndForceQuitReportTheOutcome maps the host's quit outcome to the result an extension reads.
func TestQuitAndForceQuitReportTheOutcome(t *testing.T) {
	port := &lifecycleRecorder{outcome: appfeature.QuitOutcome{Found: 1, Terminated: 1}}
	h, _, _ := lifecycleTestHandler(t, port, platform.CapabilityAppQuit)
	var quit api.QuitResult
	mustDecode(t, mustOK(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpQuit, protocol.PathArgs{Path: notesApp})), &quit)
	if quit != (api.QuitResult{WasRunning: true, Quit: true}) {
		t.Fatalf("quit = %+v; want it was running and is gone", quit)
	}

	port.outcome = appfeature.QuitOutcome{Found: 1, Killed: 1}
	var forced api.QuitResult
	mustDecode(t, mustOK(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpForceQuit, protocol.PathArgs{Path: notesApp})), &forced)
	if forced != (api.QuitResult{WasRunning: true, Quit: true, Killed: true}) || len(port.forced) != 1 {
		t.Fatalf("force quit = %+v forced %v; want a kill that ended the app", forced, port.forced)
	}

	port.outcome = appfeature.QuitOutcome{}
	var idle api.QuitResult
	mustDecode(t, mustOK(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpQuit, protocol.PathArgs{Path: notesApp})), &idle)
	if idle != (api.QuitResult{}) {
		t.Fatalf("quit of an app that is not running = %+v; want nothing to quit", idle)
	}
}

// TestQuitRefusesAnAppTheIndexDoesNotList: an extension cannot quit an arbitrary program by naming its path.
func TestQuitRefusesAnAppTheIndexDoesNotList(t *testing.T) {
	port := &lifecycleRecorder{}
	h, _, _ := lifecycleTestHandler(t, port, platform.CapabilityAppQuit)
	requireCode(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpQuit, protocol.PathArgs{Path: "/Applications/Ghost.app"}), api.CodeNotFound, "")
	if len(port.quit) != 0 {
		t.Fatalf("an unlisted app reached the port: %v", port.quit)
	}
}

// TestQuitAllNeedsTheSystemDeclaration: Quit All is a system action, as in 1.x, so desktop.apps alone is refused.
func TestQuitAllNeedsTheSystemDeclaration(t *testing.T) {
	port := &lifecycleRecorder{running: []appfeature.App{{Name: "Notes", Path: notesApp}}}
	rig := newRig(t)
	h := New(Deps{Files: real.NewPorts().FS, Clock: rig.clock, DataRoot: rig.root, Logs: rig.logs, Shell: fake.NewShell(), Capabilities: platform.NewCapabilities(platform.CapabilityAppQuit), Apps: port}, manifestFor("acme.apps", api.CapabilityDesktopApps))
	t.Cleanup(func() { _ = h.Close() })
	requireCode(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpQuitAll, nil), api.CodePermission, "system")

	system := New(Deps{Files: real.NewPorts().FS, Clock: rig.clock, DataRoot: rig.root, Logs: rig.logs, Shell: fake.NewShell(), Capabilities: platform.NewCapabilities(platform.CapabilityAppQuit), Apps: port}, manifestFor("acme.apps", api.CapabilityDesktopApps, api.CapabilitySystem))
	t.Cleanup(func() { _ = system.Close() })
	var asked []api.App
	mustDecode(t, mustOK(t, serve(t, system, protocol.ServiceDesktopApps, protocol.OpQuitAll, nil)), &asked)
	if len(asked) != 1 || asked[0].Path != notesApp {
		t.Fatalf("quit all asked %+v; want Notes", asked)
	}
}

// TestHideReachesThePortOnceTheCapabilityIsOffered: hide runs on the indexed app the call names.
func TestHideReachesThePortOnceTheCapabilityIsOffered(t *testing.T) {
	port := &lifecycleRecorder{}
	h, _, _ := lifecycleTestHandler(t, port, platform.CapabilityAppHide)
	mustOK(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpHideApp, protocol.PathArgs{Path: notesApp}))
	if len(port.hidden) != 1 || port.hidden[0] != notesApp {
		t.Fatalf("hidden = %v; want Notes", port.hidden)
	}
}

// TestScriptFailureReachesTheHUDAndTheLog: a refused AppleScript shows its reason in the HUD under the extension's
// title, writes a warning to the log, and answers the call with the same reason.
func TestScriptFailureReachesTheHUDAndTheLog(t *testing.T) {
	port := &lifecycleRecorder{err: appfeature.ScriptFailure{ExitCode: 1, Stderr: "execution error: Not authorized to send Apple events to System Events. (-1743)"}}
	h, notices, logged := lifecycleTestHandler(t, port, platform.CapabilityAppHide)
	err := h.hideApp(mustRaw(t, protocol.PathArgs{Path: notesApp}))
	if err == nil || err.Error() != automationHint {
		t.Fatalf("hide error = %v; want the automation hint", err)
	}
	if len(*notices) != 1 || !strings.HasSuffix((*notices)[0], automationHint) {
		t.Fatalf("HUD notices = %v; want the automation hint", *notices)
	}
	if !logged("AppleScript failed") {
		t.Fatal("the refused script was not logged")
	}
}

// TestOtherLifecycleFailureReachesTheHUDAndTheLog: a failure that is not a script still shows its message and is logged.
func TestOtherLifecycleFailureReachesTheHUDAndTheLog(t *testing.T) {
	port := &lifecycleRecorder{err: errors.New("could not force quit Notes")}
	h, notices, logged := lifecycleTestHandler(t, port, platform.CapabilityAppQuit)
	_, err := h.quitApp(mustRaw(t, protocol.PathArgs{Path: notesApp}), true)
	if err == nil || err.Error() != "could not force quit Notes" {
		t.Fatalf("force quit error = %v; want the message", err)
	}
	if len(*notices) != 1 || (*notices)[0] != "acme.apps: could not force quit Notes" {
		t.Fatalf("HUD notices = %v; want the message under the extension's title", *notices)
	}
	if !logged("the app action failed") {
		t.Fatal("the failed action was not logged")
	}
}

// mustDecode fails the test unless data decodes into into.
func mustDecode(t *testing.T, data json.RawMessage, into any) {
	t.Helper()
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
}

// mustRaw encodes v as the JSON arguments of a call.
func mustRaw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return raw
}
