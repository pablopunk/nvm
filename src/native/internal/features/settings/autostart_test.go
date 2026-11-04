package settings

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestLaunchAtLoginIsOfferedOnlyWhereTheOSSupportsIt hides the toggle, and refuses changes, on a system without the
// capability.
func TestLaunchAtLoginIsOfferedOnlyWhereTheOSSupportsIt(t *testing.T) {
	off := NewLaunchAtLogin(fake.NewAutostart(), false)
	if off.Supported() {
		t.Fatal("unsupported system offers the toggle")
	}
	if _, err := off.Set(true); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("Set on an unsupported system = %v, want ErrUnsupported", err)
	}
}

// TestLaunchAtLoginReportsAndChangesTheOSState turns the sign-in start on and off, and says so.
func TestLaunchAtLoginReportsAndChangesTheOSState(t *testing.T) {
	port := fake.NewAutostart()
	login := NewLaunchAtLogin(port, true)
	if login.Enabled() {
		t.Fatal("a fresh install starts at login")
	}
	message, err := login.Set(true)
	if err != nil || message != "Nevermind will start at login" {
		t.Fatalf("Set(true) = %q, %v", message, err)
	}
	if !login.Enabled() {
		t.Fatal("Enabled is false after Set(true)")
	}
	message, err = login.Set(false)
	if err != nil || message != "Nevermind will not start at login" || login.Enabled() {
		t.Fatalf("Set(false) = %q, %v, enabled %v", message, err, login.Enabled())
	}
}

// TestLaunchAtLoginFailureNamesTheChangeAndReadsOff keeps a failed change visible: the sentence names the change, and
// an unreadable state reads as off.
func TestLaunchAtLoginFailureNamesTheChangeAndReadsOff(t *testing.T) {
	port := fake.NewAutostart()
	boom := errors.New("login item refused")
	port.FailNext(fake.OpAutostartSet, boom)
	login := NewLaunchAtLogin(port, true)
	message, err := login.Set(true)
	if !errors.Is(err, boom) || message != "Could not enable start at login" {
		t.Fatalf("Set(true) = %q, %v", message, err)
	}
	port.FailNext(fake.OpAutostartEnabled, boom)
	if login.Enabled() {
		t.Fatal("an unreadable state reads as on")
	}
}
