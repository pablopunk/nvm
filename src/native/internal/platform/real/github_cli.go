package real

import "os/exec"

// GitHubCLI runs the GitHub CLI from the PATH with args and returns its standard output. Extension submission is the only
// caller; it reads the CLI's installed and signed-in state through this function.
func GitHubCLI(args ...string) (string, error) {
	out, err := exec.Command("gh", args...).Output()
	return string(out), err
}
