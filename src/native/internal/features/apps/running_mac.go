package apps

import (
	"context"
	"errors"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// psBinary lists the command of every process; the fixed arguments below never come from input.
const psBinary = "/bin/ps"

// runningMacApp reports whether an executable of the bundle at appPath is running, from the process list the way the
// 1.x code read it: a command whose path starts with the bundle's Contents folder. Any failure is an error, and the
// uninstall treats an error as running, so nothing is moved on a guess.
func runningMacApp(helper platform.Helper, appPath string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), plutilTimeout)
	defer cancel()
	result, err := helper.Run(ctx, psBinary, []string{"-axo", "comm="}, nil)
	if err != nil || result.Truncated || result.ExitCode != 0 {
		return false, errors.New("apps: could not check whether the application is running")
	}
	prefix := strings.ToLower(strings.TrimRight(appPath, "/")) + "/contents/"
	for _, line := range strings.Split(string(result.Stdout), "\n") {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), prefix) {
			return true, nil
		}
	}
	return false, nil
}
