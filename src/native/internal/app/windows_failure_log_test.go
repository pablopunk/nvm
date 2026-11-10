package app

import (
	"io"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// captureProcessLog installs an in-memory process logger for one test, so the test reads back the warnings written.
func captureProcessLog(t *testing.T) *logx.Logger {
	t.Helper()
	log, err := logx.New(nil, fake.NewClock(), "", logx.Options{Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	logx.SetDefault(log)
	t.Cleanup(func() { logx.SetDefault(nil) })
	return log
}

func loggedMessages(log *logx.Logger) []string {
	var messages []string
	for _, entry := range log.Recent(0) {
		messages = append(messages, entry.Message)
	}
	return messages
}

func TestRefusedNotificationIsLoggedInsteadOfLost(t *testing.T) {
	p := goldenPalette(t)
	log := captureProcessLog(t)
	p.ports.Notifier.(*fake.Notifier).FailNext(fake.OpNotifierShow, platform.ErrDenied)

	p.notifySystem("Left Half", "No target app was captured.")
	if !slices.Contains(loggedMessages(log), "notification.failed") {
		t.Fatalf("log = %v, want a notification.failed warning", loggedMessages(log))
	}
}

func TestFailedCaptureOfTheFrontAppIsLogged(t *testing.T) {
	p := goldenPalette(t)
	p.attachWindows()
	withoutFocusPort(p)
	p.Attach(&shownWindow{})
	log := captureProcessLog(t)
	p.ports.Windows.(*fake.WindowManager).FailNext("Frontmost", platform.ErrNotFound)

	p.onPaletteShown()
	if !slices.Contains(loggedMessages(log), "window.capture.failed") {
		t.Fatalf("log = %v, want a window.capture.failed warning", loggedMessages(log))
	}
}
