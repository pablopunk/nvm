package real

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestFrontmostIdentityIsTheBundleOfAnotherApp(t *testing.T) {
	app := platform.WindowApp{PID: 412, BundleID: "com.apple.finder"}
	if got := frontmostIdentityOf(app, true, 9000); got != "com.apple.finder" {
		t.Fatalf("identity = %q, want the front app's bundle", got)
	}
}

func TestFrontmostIdentityIsUnknownWhenNevermindHasTheKeyboardOrNoAppIsInFront(t *testing.T) {
	app := platform.WindowApp{PID: 9000, BundleID: "com.pablopunk.nvm.next"}
	if got := frontmostIdentityOf(app, true, 9000); got != "" {
		t.Fatalf("own identity = %q, want unknown", got)
	}
	if got := frontmostIdentityOf(platform.WindowApp{}, false, 9000); got != "" {
		t.Fatalf("no app identity = %q, want unknown", got)
	}
}
