package real

import (
	"os"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// macosHelperPath returns the helper bundled with the app, else the one beside the executable.
func macosHelperPath() string {
	executableDir := ""
	if executable, err := os.Executable(); err == nil {
		executableDir = filepath.Dir(executable)
	}
	return chooseMacosHelperPath(existingBundledResource(macosHelperName), executableDir)
}

func newMacHelper(helper platform.Helper) macHelper {
	return macHelper{helper: helper, binary: macosHelperPath()}
}
