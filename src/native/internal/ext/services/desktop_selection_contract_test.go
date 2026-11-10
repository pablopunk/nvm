package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

type contractSelection struct {
	calls int
	err   error
}

func (s *contractSelection) Text() (string, bool, error)             { s.calls++; return "", false, s.err }
func (s *contractSelection) Replace(string) (bool, error)            { s.calls++; return false, s.err }
func (s *contractSelection) Undo() error                             { s.calls++; return s.err }
func (s *contractSelection) Files(context.Context) ([]string, error) { s.calls++; return nil, s.err }

func TestSelectionPermissionAndEmpty(t *testing.T) {
	s := &contractSelection{err: platform.ErrDenied}
	h := desktopHandler(t, Deps{Selection: s}, api.CapabilityDesktopSelection, api.CapabilityDesktopFiles, api.CapabilityDesktopKeyboard)
	for _, op := range []string{protocol.OpText, protocol.OpFiles} {
		requireCode(t, serve(t, h, protocol.ServiceDesktopSelection, op, nil), api.CodePermission, "Allow Nevermind")
	}
	requireCode(t, serve(t, h, protocol.ServiceDesktopKeyboard, protocol.OpUndo, nil), api.CodePermission, "Allow Nevermind")
	requireCode(t, serve(t, h, protocol.ServiceDesktopSelection, protocol.OpReplace, protocol.ReplaceTextArgs{Text: "x"}), api.CodePermission, "Allow Nevermind")
	s.err = platform.ErrUnsupported
	requireCode(t, serve(t, h, protocol.ServiceDesktopSelection, protocol.OpText, nil), api.CodeUnavailable, "not available")
	s.err = nil
	result, err := h.serveDesktopSelection(context.Background(), protocol.ServiceCall{Op: protocol.OpText})
	if err != nil || result != (protocol.SelectionTextResult{Text: "", Found: false}) {
		t.Fatalf("empty selection: %#v, %v", result, err)
	}
}

func TestCancelledSelectionDoesNotAdmitEffect(t *testing.T) {
	s := &contractSelection{}
	h := desktopHandler(t, Deps{Selection: s}, api.CapabilityDesktopSelection, api.CapabilityDesktopKeyboard, api.CapabilityDesktopFiles)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, op := range []string{protocol.OpText, protocol.OpReplace} {
		args := []byte(`{"text":"x"}`)
		_, err := h.serveDesktopSelection(ctx, protocol.ServiceCall{Op: op, Args: args})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%s: %v", op, err)
		}
	}
	if _, err := h.serveDesktopSelection(ctx, protocol.ServiceCall{Op: protocol.OpFiles}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := h.serveDesktopKeyboard(ctx, protocol.ServiceCall{Op: protocol.OpUndo}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if s.calls != 0 {
		t.Fatalf("admitted %d effects", s.calls)
	}
}

func TestCancelledPasteAndTypingDoNotAdmitEffect(t *testing.T) {
	recorder := &pasteRecorder{}
	h := desktopHandler(t, Deps{Paste: recorder}, api.CapabilityDesktopClipboard, api.CapabilityDesktopKeyboard)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, action := range []api.Action{{Kind: api.ActionPasteText, Text: "x"}, {Kind: api.ActionTypeText, Text: "x"}} {
		if err := h.performPasteContext(ctx, action); !errors.Is(err, context.Canceled) {
			t.Fatalf("%s: %v", action.Kind, err)
		}
	}
	if len(recorder.pasted) != 0 || len(recorder.typed) != 0 {
		t.Fatal("cancelled effects were admitted")
	}
}

type cancelReadFS struct {
	platform.FS
	cancel context.CancelFunc
}

func (fs cancelReadFS) ReadFile(path string) ([]byte, error) {
	data, err := fs.FS.ReadFile(path)
	fs.cancel()
	return data, err
}

func TestCancellationDuringImagePreparationDoesNotPaste(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(path, pngSignature, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	recorder := &pasteRecorder{}
	h := desktopHandler(t, Deps{Paste: recorder, Files: cancelReadFS{FS: real.NewPorts().FS, cancel: cancel}}, api.CapabilityDesktopClipboard, api.CapabilityDesktopKeyboard)
	action := api.Paste("Image", api.ClipboardContent{Type: "image", ImagePath: path}, api.PasteOptions{})
	if err := h.performPasteContext(ctx, action); !errors.Is(err, context.Canceled) {
		t.Fatalf("paste: %v", err)
	}
	if len(recorder.pasted) != 0 {
		t.Fatal("cancelled paste was admitted")
	}
}

func TestKeyboardAndPasteDenialIsPermission(t *testing.T) {
	recorder := &pasteRecorder{err: platform.ErrDenied}
	h := desktopHandler(t, Deps{Paste: recorder}, api.CapabilityDesktopClipboard, api.CapabilityDesktopKeyboard)
	requireCode(t, serve(t, h, protocol.ServiceDesktopKeyboard, protocol.OpTypeText, protocol.TypeTextArgs{Text: "x"}), api.CodePermission, "Allow Nevermind")
	requireCode(t, serve(t, h, protocol.ServiceDesktopPaste, protocol.OpPerform, api.PasteText("Paste", "x")), api.CodePermission, "Allow Nevermind")
}

func TestClosedHostDuringImagePreparationDoesNotPaste(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(path, pngSignature, 0600); err != nil {
		t.Fatal(err)
	}
	recorder := &pasteRecorder{}
	var h *Handler
	files := cancelReadFS{FS: real.NewPorts().FS, cancel: func() { _ = h.Close() }}
	h = desktopHandler(t, Deps{Paste: recorder, Files: files}, api.CapabilityDesktopClipboard, api.CapabilityDesktopKeyboard)
	action := api.Paste("Image", api.ClipboardContent{Type: "image", ImagePath: path}, api.PasteOptions{})
	if err := h.PerformPaste(action); !errors.Is(err, api.ErrNotAvailable) {
		t.Fatalf("closed host paste: %v", err)
	}
	if len(recorder.pasted) != 0 {
		t.Fatal("closed host admitted paste after content preparation")
	}
}

func TestStoppedLifetimeDoesNotAdmitPaste(t *testing.T) {
	recorder := &pasteRecorder{}
	h := desktopHandler(t, Deps{Paste: recorder}, api.CapabilityDesktopClipboard, api.CapabilityDesktopKeyboard)
	lifetime, stop := context.WithCancel(context.Background())
	stop()
	ctx := protocol.WithLifetime(context.Background(), lifetime)
	if err := h.performPasteContext(ctx, api.PasteText("Paste", "x")); !errors.Is(err, context.Canceled) {
		t.Fatalf("stopped lifetime paste: %v", err)
	}
	if len(recorder.pasted) != 0 {
		t.Fatal("stopped lifetime admitted paste")
	}
}
