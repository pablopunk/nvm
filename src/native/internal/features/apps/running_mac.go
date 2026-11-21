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
// 1.x code read it. Any failure is an error, and the uninstall treats an error as running, so nothing is moved on a guess.
func runningMacApp(helper platform.Helper, appPath string) (bool, error) {
	running, err := runningMacApps(helper, []App{{Path: appPath}})
	return running[appPath], err
}

// runningMacApps reports which candidates have an executable running from their bundle. One process listing answers
// for every candidate, so the running state of all installed apps costs one call.
func runningMacApps(helper platform.Helper, candidates []App) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), plutilTimeout)
	defer cancel()
	result, err := helper.Run(ctx, psBinary, []string{"-axo", "comm="}, nil)
	if err != nil || result.Truncated || result.ExitCode != 0 {
		return nil, errors.New("apps: could not check whether the application is running")
	}
	commands := strings.Split(strings.ToLower(string(result.Stdout)), "\n")
	running := map[string]bool{}
	for _, app := range candidates {
		prefix := strings.ToLower(strings.TrimRight(app.Path, "/")) + "/contents/"
		for _, command := range commands {
			if strings.HasPrefix(strings.TrimSpace(command), prefix) {
				running[app.Path] = true
				break
			}
		}
	}
	return running, nil
}

// macRunningDetect is the running-state detector on macOS, where the Processes port has no listing. Other systems keep
// the port's detector, so it returns nil for them.
func macRunningDetect(goos string, helper platform.Helper) func([]App) (map[string]bool, error) {
	if goos != "darwin" {
		return nil
	}
	return func(candidates []App) (map[string]bool, error) {
		return runningMacApps(helper, candidates)
	}
}
