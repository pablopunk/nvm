package app

import (
	"os"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/child"
)

// TestMain makes the test binary serve as the extension child when a test starts a real extension.
func TestMain(m *testing.M) {
	if code, handled := child.Main(os.Args); handled {
		os.Exit(code)
	}
	os.Exit(m.Run())
}
