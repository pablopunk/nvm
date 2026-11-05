package services

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
)

// stubWindows is a window engine with one display and one window, so the placement path can be asserted.
type stubWindows struct{ focused, placed []api.WindowHandle }

func (s *stubWindows) Displays(context.Context) ([]api.DisplayInfo, error) {
	return []api.DisplayInfo{{ID: "main", Primary: true}}, nil
}

func (s *stubWindows) Windows(_ context.Context, query api.WindowQuery) ([]api.WindowInfo, error) {
	return []api.WindowInfo{{Handle: "w1", AppName: "Notes", Title: query.Title}}, nil
}

func (s *stubWindows) Focus(_ context.Context, handle api.WindowHandle) (api.FocusResult, error) {
	s.focused = append(s.focused, handle)
	return api.FocusResult{OK: true}, nil
}

func (s *stubWindows) ApplyTo(_ context.Context, handle api.WindowHandle, _ api.WindowLayout, _ api.WindowManagementOptions) (api.ApplyResult, error) {
	s.placed = append(s.placed, handle)
	return api.ApplyResult{OK: true, Outcome: "applied"}, nil
}

type stubShortcuts struct{}

func (stubShortcuts) List() []api.ShortcutRecord {
	return []api.ShortcutRecord{{ActionID: "palette", Accelerator: "Command+Alt+K", Source: "user"}}
}

func (stubShortcuts) Aliases(string) map[string][]string { return nil }

func (stubShortcuts) Palette() api.ShortcutRecord {
	return api.ShortcutRecord{ActionID: "palette", Accelerator: "Command+Alt+K", Source: "user"}
}

// desktopHost is every host port the desktop services use, with the fakes a test inspects.
type desktopHost struct {
	shell     *fake.Shell
	clipboard *fake.Clipboard
	text      *fake.TextRecognizer
	windows   *stubWindows
	history   *clipboard.History
}

func newDesktopHost(t *testing.T, caps ...platform.Capability) (desktopHost, Deps) {
	t.Helper()
	clock := fake.NewClock()
	host := desktopHost{
		shell:     fake.NewShell(),
		clipboard: fake.NewClipboard(),
		text:      fake.NewTextRecognizer(),
		windows:   &stubWindows{},
	}
	history, err := clipboard.Open(clipboard.NewMemoryPersistence(), clock, clipboard.Retention{MaxEntries: 100, MaxAge: 24 * time.Hour, MaxImageBytes: 1 << 30, MaxMemoryBytes: 1 << 30})
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	host.history = history
	deps := Deps{
		Clock:        clock,
		Clipboard:    host.clipboard,
		Shell:        host.shell,
		Windows:      host.windows,
		Shortcuts:    stubShortcuts{},
		OCR:          ocr.New(ocr.Deps{Text: host.text, Capture: fake.NewScreenCapturer(), FS: fake.NewFS(), TempDir: t.TempDir()}),
		OS:           "darwin",
		Capabilities: platform.NewCapabilities(caps...),
		History:      func() *clipboard.History { return host.history },
	}
	return host, deps
}

func desktopHandler(t *testing.T, deps Deps, capabilities ...api.Capability) *Handler {
	t.Helper()
	deps.DataRoot = t.TempDir()
	deps.Files = fake.NewFS()
	deps.Logs = nil
	handler := New(deps, manifestFor("acme.desktop", capabilities...))
	t.Cleanup(func() { _ = handler.Close() })
	return handler
}

// serveRun sends one call under a palette run, the only context destructive desktop calls accept.
func serveRun(t *testing.T, h *Handler, service, op string, args any) protocol.ServiceReply {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	ctx := protocol.WithRequestMethod(context.Background(), protocol.MethodRun)
	return h.Serve(ctx, protocol.ServiceCall{CallID: 1, Service: service, Op: op, Args: raw})
}

func TestClipboardReadsAndWritesText(t *testing.T) {
	host, deps := newDesktopHost(t)
	h := desktopHandler(t, deps, api.CapabilityDesktopClipboard)
	mustOK(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpWriteText, protocol.WriteTextArgs{Text: "hello"}))
	var text string
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpReadText, nil)), &text); err != nil || text != "hello" {
		t.Fatalf("read back %q (%v), want hello", text, err)
	}
	if got, _ := host.clipboard.ReadText(); got != "hello" {
		t.Fatalf("clipboard holds %q", got)
	}
}

func TestConcealedWriteRefusesWhenTheClipboardCannotMarkIt(t *testing.T) {
	host, deps := newDesktopHost(t)
	deps.Clipboard = plainClipboard{host.clipboard}
	h := desktopHandler(t, deps, api.CapabilityDesktopClipboard)
	requireCode(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpWriteText, protocol.WriteTextArgs{Text: "secret", Concealed: true}), api.CodeUnavailable, "history")
	if got, _ := host.clipboard.ReadText(); got == "secret" {
		t.Fatal("an unmarked concealed write reached the clipboard")
	}
}

// plainClipboard hides the concealed write, the way a clipboard without the marker does.
type plainClipboard struct{ platform.Clipboard }

func TestAppsLaunchOnlyTheInstalledIndex(t *testing.T) {
	host, deps := newDesktopHost(t)
	notes := testpath.Abs("Applications", "Notes.app")
	host.shell.SetApps([]platform.App{{Name: "Notes", ID: "com.example.notes", Path: notes}}, nil)
	h := desktopHandler(t, deps, api.CapabilityDesktopApps)
	requireCode(t, serveRun(t, h, protocol.ServiceDesktopApps, protocol.OpLaunch, protocol.PathArgs{Path: testpath.Abs("usr", "bin", "evil")}), api.CodeNotFound, "installed app")
	mustOK(t, serveRun(t, h, protocol.ServiceDesktopApps, protocol.OpLaunch, protocol.PathArgs{Path: notes}))
	if calls := host.shell.Calls(); len(calls) == 0 || calls[len(calls)-1].Arg != notes {
		t.Fatalf("launch calls %+v", calls)
	}
}

func TestAppSearchMatchesNamesIgnoringCase(t *testing.T) {
	host, deps := newDesktopHost(t)
	host.shell.SetApps([]platform.App{{Name: "Notes", Path: "/Applications/Notes.app"}, {Name: "Mail", Path: "/Applications/Mail.app"}}, nil)
	h := desktopHandler(t, deps, api.CapabilityDesktopApps)
	var apps []api.App
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopApps, protocol.OpSearch, protocol.QueryArgs{Query: "NOT"})), &apps); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(apps) != 1 || apps[0].Name != "Notes" {
		t.Fatalf("search returned %+v", apps)
	}
}

func TestFilesNeedAbsolutePathsAndDeclaration(t *testing.T) {
	host, deps := newDesktopHost(t)
	open := desktopHandler(t, deps, api.CapabilityDesktopFiles)
	requireCode(t, serve(t, open, protocol.ServiceDesktopFiles, protocol.OpOpen, protocol.PathArgs{Path: "notes.txt"}), api.CodeInvalid, "absolute")
	path := filepath.Join(t.TempDir(), "notes.txt")
	mustOK(t, serve(t, open, protocol.ServiceDesktopFiles, protocol.OpReveal, protocol.PathArgs{Path: path}))
	if calls := host.shell.Calls(); len(calls) != 1 || calls[0].Op != "Reveal" {
		t.Fatalf("calls %+v", calls)
	}
	undeclared := desktopHandler(t, deps)
	requireCode(t, serve(t, undeclared, protocol.ServiceDesktopFiles, protocol.OpOpen, protocol.PathArgs{Path: path}), api.CodePermission, "desktop.files")
}

func TestOpenExternalAcceptsOnlyWebLinks(t *testing.T) {
	_, deps := newDesktopHost(t)
	h := desktopHandler(t, deps, api.CapabilityDesktopFiles)
	mustOK(t, serve(t, h, protocol.ServiceDesktopShell, protocol.OpOpenExternal, protocol.URLArgs{URL: "https://maze.design"}))
	requireCode(t, serve(t, h, protocol.ServiceDesktopShell, protocol.OpOpenExternal, protocol.URLArgs{URL: "file:///etc/passwd"}), api.CodeInvalid, "http")
}

func TestWindowsListAndPlaceThroughTheEngine(t *testing.T) {
	host, deps := newDesktopHost(t, platform.CapabilityWindowManagement)
	h := desktopHandler(t, deps, api.CapabilityWindowManagement)
	var windows []api.WindowInfo
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopWindows, protocol.OpWindows, api.WindowQuery{Title: "draft"})), &windows); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(windows) != 1 || windows[0].Title != "draft" {
		t.Fatalf("windows %+v", windows)
	}
	layout := api.WindowLayout{Kind: api.LayoutFraction, Fraction: &api.Rect{Width: 0.5, Height: 1}}
	mustOK(t, serve(t, h, protocol.ServiceDesktopWindows, protocol.OpApplyTo, protocol.ApplyToArgs{Handle: "w1", Layout: layout}))
	if len(host.windows.placed) != 1 {
		t.Fatalf("placed %v", host.windows.placed)
	}
	requireCode(t, serve(t, h, protocol.ServiceDesktopWindows, protocol.OpApplyTo, protocol.ApplyToArgs{Handle: "w1", Layout: api.WindowLayout{Kind: "bogus"}}), api.CodeInvalid, "")
}

func TestClipboardHistoryListsAndOnlyRemovesFromAPaletteRun(t *testing.T) {
	host, deps := newDesktopHost(t)
	now := fake.NewClock().Now()
	entry := clipboard.Entry{ID: clipboard.TextID("draft"), Kind: clipboard.KindText, Text: "draft", CreatedAt: now, UsedAt: now}
	if err := host.history.Remember(entry); err != nil {
		t.Fatalf("remember: %v", err)
	}
	deps.Confirm = func(context.Context, Question) (bool, error) { return true, nil }
	h := desktopHandler(t, deps, api.CapabilityClipboardHistory)
	var entries []api.ClipboardEntry
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceClipboardHistory, protocol.OpList, api.ClipboardQuery{Text: "dra"})), &entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(entries) != 1 || entries[0].Text != "draft" {
		t.Fatalf("list %+v", entries)
	}
	requireCode(t, serve(t, h, protocol.ServiceClipboardHistory, protocol.OpRemove, protocol.IDsArgs{IDs: []string{entry.ID}}), api.CodePermission, "palette action")
	var removed int
	if err := jsonInto(mustOK(t, serveRun(t, h, protocol.ServiceClipboardHistory, protocol.OpRemove, protocol.IDsArgs{IDs: []string{entry.ID}})), &removed); err != nil || removed != 1 {
		t.Fatalf("removed %d (%v), want 1", removed, err)
	}
}

func TestOCRReadsImagesAndIsRateLimited(t *testing.T) {
	host, deps := newDesktopHost(t, platform.CapabilityTextRecognize)
	image := filepath.Join(t.TempDir(), "receipt.png")
	host.text.Script(image, platform.TextResult{Text: "total 4"})
	h := desktopHandler(t, deps, api.CapabilityOCR)
	var result api.OCRResult
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceOCR, protocol.OpImage, protocol.OCRImageArgs{Input: api.OCRInput{Source: image}})), &result); err != nil || result.Text != "total 4" {
		t.Fatalf("ocr %+v (%v)", result, err)
	}
	requireCode(t, serve(t, h, protocol.ServiceOCR, protocol.OpScreen, protocol.OCRScreenArgs{}), api.CodeUnavailable, "screen")
	for range ocrReadsPerMinute - 1 {
		serve(t, h, protocol.ServiceOCR, protocol.OpImage, protocol.OCRImageArgs{Input: api.OCRInput{Source: image}})
	}
	requireCode(t, serve(t, h, protocol.ServiceOCR, protocol.OpImage, protocol.OCRImageArgs{Input: api.OCRInput{Source: image}}), api.CodeQuota, "rate limited")
}

// TestHasCapabilityAnswersSelectionCopy checks the name the AI commands built-in asks before it offers its fix: an answer
// of false there hides the command, so the name must be one the host maps.
func TestHasCapabilityAnswersSelectionCopy(t *testing.T) {
	_, deps := newDesktopHost(t, platform.CapabilitySelectionCopy)
	h := desktopHandler(t, deps)
	var supported bool
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceSystem, protocol.OpHasCapability, protocol.CapabilityArgs{Name: "selection.copy"})), &supported); err != nil || !supported {
		t.Fatalf("selection.copy %v (%v)", supported, err)
	}
}

func TestSystemAndShortcutsNeedOnlyTheirDeclarations(t *testing.T) {
	_, deps := newDesktopHost(t, platform.CapabilityWindowManagement)
	bare := desktopHandler(t, deps)
	var osName string
	if err := jsonInto(mustOK(t, serve(t, bare, protocol.ServiceSystem, protocol.OpOS, nil)), &osName); err != nil || osName != "darwin" {
		t.Fatalf("os %q (%v)", osName, err)
	}
	var supported bool
	if err := jsonInto(mustOK(t, serve(t, bare, protocol.ServiceSystem, protocol.OpHasCapability, protocol.CapabilityArgs{Name: "window-management"})), &supported); err != nil || !supported {
		t.Fatalf("window-management %v (%v)", supported, err)
	}
	requireCode(t, serve(t, bare, protocol.ServiceShortcuts, protocol.OpList, nil), api.CodePermission, "shortcuts")
	declared := desktopHandler(t, deps, api.CapabilityShortcuts)
	var palette api.ShortcutRecord
	if err := jsonInto(mustOK(t, serve(t, declared, protocol.ServiceShortcuts, protocol.OpPalette, nil)), &palette); err != nil || palette.Accelerator == "" {
		t.Fatalf("palette %+v (%v)", palette, err)
	}
}

// TestHostileExtensionWithNoDeclarationsReachesNoDesktopService calls every desktop service without declaring it and
// requires a permission refusal from each, so a manifest that declares nothing can use none of them.
func TestHostileExtensionWithNoDeclarationsReachesNoDesktopService(t *testing.T) {
	host, deps := newDesktopHost(t, platform.CapabilityWindowManagement, platform.CapabilityTextRecognize)
	host.shell.SetApps([]platform.App{{Name: "Notes", Path: "/Applications/Notes.app"}}, nil)
	h := desktopHandler(t, deps)
	calls := []struct {
		service, op string
		args        any
	}{
		{protocol.ServiceDesktopClipboard, protocol.OpReadText, nil},
		{protocol.ServiceDesktopClipboard, protocol.OpWriteText, protocol.WriteTextArgs{Text: "x"}},
		{protocol.ServiceDesktopApps, protocol.OpList, nil},
		{protocol.ServiceDesktopApps, protocol.OpLaunch, protocol.PathArgs{Path: "/Applications/Notes.app"}},
		{protocol.ServiceDesktopFiles, protocol.OpOpen, protocol.PathArgs{Path: "/tmp/x"}},
		{protocol.ServiceDesktopShell, protocol.OpOpenExternal, protocol.URLArgs{URL: "https://example.com"}},
		{protocol.ServiceDesktopWindows, protocol.OpWindows, api.WindowQuery{}},
		{protocol.ServiceClipboardHistory, protocol.OpList, api.ClipboardQuery{}},
		{protocol.ServiceClipboardHistory, protocol.OpClear, protocol.ClearArgs{}},
		{protocol.ServiceOCR, protocol.OpImage, protocol.OCRImageArgs{Input: api.OCRInput{Source: "/tmp/x.png"}}},
		{protocol.ServiceShortcuts, protocol.OpList, nil},
	}
	for _, call := range calls {
		reply := serveRun(t, h, call.service, call.op, call.args)
		if reply.Failure == nil || reply.Failure.Code != api.CodePermission {
			t.Errorf("%s.%s without a declaration: %+v, want permission", call.service, call.op, reply.Failure)
		}
	}
	if len(host.shell.Calls()) != 0 || len(host.windows.placed) != 0 {
		t.Fatalf("an undeclared call reached the host: shell %+v", host.shell.Calls())
	}
}
