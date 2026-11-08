package services

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/selection"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// pasteRecorder is the paste engine a test inspects: what it was asked to paste and type, and the failure it returns.
type pasteRecorder struct {
	pasted  []selection.Content
	options []selection.PasteOptions
	typed   []string
	delays  []time.Duration
	err     error
}

func (p *pasteRecorder) Paste(content selection.Content, options selection.PasteOptions) error {
	if p.err != nil {
		return p.err
	}
	p.pasted = append(p.pasted, content)
	p.options = append(p.options, options)
	return nil
}

func (p *pasteRecorder) TypeText(text string, delay time.Duration) error {
	if p.err != nil {
		return p.err
	}
	p.typed = append(p.typed, text)
	p.delays = append(p.delays, delay)
	return nil
}

// pasteHandler opens an extension's services over the paste recorder, with the logs the test reads back.
func pasteHandler(t *testing.T, engine PasteEngine, capabilities ...api.Capability) (*Handler, *logx.Logger) {
	t.Helper()
	r := newRig(t)
	ports := real.NewPorts()
	h := New(Deps{
		Files:     ports.FS,
		Clock:     r.clock,
		DataRoot:  r.root,
		Clipboard: r.clipboard,
		Logs:      r.logs,
		Paste:     engine,
	}, manifestFor("acme.paste", capabilities...))
	t.Cleanup(func() { _ = h.Close() })
	return h, r.logs
}

var pastingCapabilities = []api.Capability{api.CapabilityDesktopClipboard, api.CapabilityDesktopKeyboard}

// TestPasteNeedsTheClipboardAndKeyboardDeclarations: a paste writes the clipboard and sends a keystroke, so each
// capability is required, and a refusal never reaches the engine.
func TestPasteNeedsTheClipboardAndKeyboardDeclarations(t *testing.T) {
	engine := &pasteRecorder{}
	keyboardOnly, _ := pasteHandler(t, engine, api.CapabilityDesktopKeyboard)
	if err := keyboardOnly.PerformPaste(api.PasteText("Paste", "hi")); err == nil || !strings.Contains(err.Error(), "desktop.clipboard") {
		t.Fatalf("paste without the clipboard capability = %v; want a refusal naming it", err)
	}
	clipboardOnly, _ := pasteHandler(t, engine, api.CapabilityDesktopClipboard)
	if err := clipboardOnly.PerformPaste(api.PasteText("Paste", "hi")); err == nil || !strings.Contains(err.Error(), "desktop.keyboard") {
		t.Fatalf("paste without the keyboard capability = %v; want a refusal naming it", err)
	}
	if len(engine.pasted) != 0 {
		t.Fatalf("refused pastes reached the engine: %+v", engine.pasted)
	}
}

// TestPasteTextPastesPlainTextWithTheOneXDefaults: paste text writes plain text, keeps the clipboard, and waits the
// 250 ms default before a restore, as 1.x did.
func TestPasteTextPastesPlainTextWithTheOneXDefaults(t *testing.T) {
	engine := &pasteRecorder{}
	h, _ := pasteHandler(t, engine, pastingCapabilities...)
	if err := h.PerformPaste(api.PasteText("Paste", "hi")); err != nil {
		t.Fatalf("paste: %v", err)
	}
	if got := engine.pasted; len(got) != 1 || got[0].Kind != selection.KindText || got[0].Text != "hi" || got[0].HTML != "" {
		t.Fatalf("pasted = %+v; want the plain text hi", engine.pasted)
	}
	if got := engine.options[0]; got.RestoreClipboard || got.Concealed || got.RestoreDelay != 250*time.Millisecond {
		t.Fatalf("options = %+v; want no restore, not concealed, 250 ms", got)
	}
}

// TestPasteOptionsCarryToTheEngine: the restore flag, the restore delay clamped to the 1.x 50 ms floor, and the expected
// frontmost app travel to the engine. The engine, not the service, makes a restored clipboard concealed.
func TestPasteOptionsCarryToTheEngine(t *testing.T) {
	engine := &pasteRecorder{}
	h, _ := pasteHandler(t, engine, pastingCapabilities...)
	action := api.Paste("Paste", api.ClipboardContent{Type: "text", Text: "hi"}, api.PasteOptions{
		RestoreClipboard:    true,
		RestoreDelayMs:      10,
		ExpectedFrontmostID: "com.apple.TextEdit",
	})
	if err := h.PerformPaste(action); err != nil {
		t.Fatalf("paste: %v", err)
	}
	want := selection.PasteOptions{RestoreClipboard: true, RestoreDelay: 50 * time.Millisecond, ExpectedAppID: "com.apple.TextEdit"}
	if engine.options[0] != want {
		t.Fatalf("options = %+v; want %+v", engine.options[0], want)
	}
}

// TestRichPasteNeedsMarkupAndOptInForHTML: HTML is dropped unless the caller turns plain-text-only off, and an HTML
// paste without its markup is refused.
func TestRichPasteNeedsMarkupAndOptInForHTML(t *testing.T) {
	engine := &pasteRecorder{}
	h, _ := pasteHandler(t, engine, pastingCapabilities...)
	if err := h.PerformPaste(api.Paste("Paste", api.ClipboardContent{Type: "html", Text: "hi"}, api.PasteOptions{})); err == nil {
		t.Fatal("an html paste without markup was accepted")
	}
	plainText := api.Paste("Paste", api.ClipboardContent{Type: "text", Text: "hi", HTML: "<b>hi</b>"}, api.PasteOptions{})
	if err := h.PerformPaste(plainText); err != nil {
		t.Fatalf("plain text paste: %v", err)
	}
	off := false
	rich := api.Paste("Paste", api.ClipboardContent{Type: "text", Text: "hi", HTML: "<b>hi</b>"}, api.PasteOptions{PlainTextOnly: &off})
	if err := h.PerformPaste(rich); err != nil {
		t.Fatalf("rich text paste: %v", err)
	}
	if engine.pasted[0].HTML != "" || engine.pasted[1].HTML != "<b>hi</b>" {
		t.Fatalf("pasted = %+v; want HTML only when plain-text-only is off", engine.pasted)
	}
}

// TestFilesAndImagesAreCheckedBeforeTheClipboard: paths must be absolute, images must be PNG, and a PNG file is read
// from disk.
func TestFilesAndImagesAreCheckedBeforeTheClipboard(t *testing.T) {
	engine := &pasteRecorder{}
	h, _ := pasteHandler(t, engine, pastingCapabilities...)
	if err := h.PerformPaste(api.Paste("Files", api.ClipboardContent{Type: "files", Paths: []string{"relative.txt"}}, api.PasteOptions{})); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative file path = %v; want an absolute-path refusal", err)
	}
	jpeg := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte("\xff\xd8\xff"))
	if err := h.PerformPaste(api.Paste("Image", api.ClipboardContent{Type: "image", ImageDataURL: jpeg}, api.PasteOptions{})); err == nil {
		t.Fatal("a JPEG data URL was accepted as an image")
	}
	png := []byte("\x89PNG\r\n\x1a\nbody")
	path := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(path, png, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.PerformPaste(api.Paste("Image", api.ClipboardContent{Type: "image", ImagePath: path}, api.PasteOptions{})); err != nil {
		t.Fatalf("PNG file paste: %v", err)
	}
	if string(engine.pasted[0].PNG) != string(png) || engine.pasted[0].Kind != selection.KindImage {
		t.Fatalf("pasted = %+v; want the PNG bytes of the file", engine.pasted[0])
	}
}

// TestPasteFailuresReachTheHUDAndTheLog: a failed paste shows its palette message and writes a warning to the log.
func TestPasteFailuresReachTheHUDAndTheLog(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		message string
	}{
		{"focus moved", platform.ErrConflict, "The focused app changed, so nothing was pasted."},
		{"keyboard denied", platform.ErrDenied, "Allow Nevermind to control the keyboard in System Settings."},
		{"concealed refused", selection.ErrConcealedContent, selection.ErrConcealedContent.Error()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, logs := pasteHandler(t, &pasteRecorder{err: tc.err}, pastingCapabilities...)
			err := h.PerformPaste(api.PasteText("Paste", "hi"))
			if err == nil || err.Error() != tc.message {
				t.Fatalf("error = %v; want %q", err, tc.message)
			}
			if !hasWarning(logs, "the paste action failed") {
				t.Fatal("the failed paste was not logged as a warning")
			}
		})
	}
}

// TestTypingThroughTheKeyboardService: the child types through desktop.keyboard, with the delay the call names.
func TestTypingThroughTheKeyboardService(t *testing.T) {
	engine := &pasteRecorder{}
	h, _ := pasteHandler(t, engine, api.CapabilityDesktopKeyboard)
	mustOK(t, serve(t, h, protocol.ServiceDesktopKeyboard, protocol.OpTypeText, protocol.TypeTextArgs{Text: "abc", Options: api.TypeOptions{DelayMs: 5}}))
	if len(engine.typed) != 1 || engine.typed[0] != "abc" || engine.delays[0] != 5*time.Millisecond {
		t.Fatalf("typed = %v delays = %v; want abc after 5 ms", engine.typed, engine.delays)
	}
}

// TestTypingWithoutTheKeyboardDeclarationIsRefused: a call without desktop.keyboard never reaches the engine.
func TestTypingWithoutTheKeyboardDeclarationIsRefused(t *testing.T) {
	engine := &pasteRecorder{}
	h, _ := pasteHandler(t, engine)
	requireCode(t, serve(t, h, protocol.ServiceDesktopKeyboard, protocol.OpTypeText, protocol.TypeTextArgs{Text: "abc"}), api.CodePermission, "desktop.keyboard")
	if len(engine.typed) != 0 {
		t.Fatalf("refused typing reached the engine: %v", engine.typed)
	}
}

// TestTypeActionDelayIsClampedToOneSecond keeps a runaway delay from holding a typing action for minutes.
func TestTypeActionDelayIsClampedToOneSecond(t *testing.T) {
	engine := &pasteRecorder{}
	h, _ := pasteHandler(t, engine, api.CapabilityDesktopKeyboard)
	if err := h.PerformPaste(api.TypeText("Type", "x", api.TypeOptions{DelayMs: 5000})); err != nil {
		t.Fatalf("type: %v", err)
	}
	if engine.delays[0] != time.Second {
		t.Fatalf("delay = %v; want 1s", engine.delays[0])
	}
}

// TestUnknownPasteOperationIsInvalid keeps the desktop.paste service to the one operation the palette sends.
func TestUnknownPasteOperationIsInvalid(t *testing.T) {
	h, _ := pasteHandler(t, &pasteRecorder{}, pastingCapabilities...)
	requireCode(t, serve(t, h, protocol.ServiceDesktopPaste, "nope", nil), api.CodeInvalid, "")
}

// hasWarning reports whether the log holds a warning with message.
func hasWarning(logs *logx.Logger, message string) bool {
	for _, entry := range logs.Recent(0) {
		if entry.Level == logx.Warn && entry.Message == message {
			return true
		}
	}
	return false
}

func TestPasteRevokedBeforeIrreversibleAdmission(t *testing.T) {
	for _, action := range []api.Action{
		api.PasteText("Paste", "fixture"),
		api.TypeText("Type", "fixture", api.TypeOptions{}),
	} {
		t.Run(string(action.Kind), func(t *testing.T) {
			engine := &pasteRecorder{}
			h, _ := pasteHandler(t, engine, pastingCapabilities...)
			entered, release := make(chan struct{}), make(chan struct{})
			var revoked atomic.Bool
			ctx := WithPasteAdmission(context.Background(), func() bool {
				close(entered)
				<-release
				return !revoked.Load()
			})
			done := make(chan error, 1)
			go func() { done <- h.performPaste(ctx, action) }()
			<-entered
			revoked.Store(true)
			close(release)
			if err := <-done; err == nil {
				t.Fatal("revoked operation succeeded")
			}
			if len(engine.pasted) != 0 || len(engine.typed) != 0 {
				t.Fatal("revoked operation reached engine")
			}
		})
	}
}
