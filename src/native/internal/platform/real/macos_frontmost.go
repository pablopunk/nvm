package real

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// lsappinfoPath is the Launch Services tool the Electron app asks which application is in front.
const lsappinfoPath = "/usr/bin/lsappinfo"

var (
	lsappinfoBundleID = regexp.MustCompile(`(?:"CFBundleIdentifier"|\bbundleID)\s*=\s*"([^"]+)"`)
	lsappinfoPID      = regexp.MustCompile(`(?:"pid"|\bpid)\s*=\s*(\d+)`)
)

// frontmostApp names the application in front, as Launch Services reports it. It lists no windows, so it is quick
// enough for the palette's summon, and a system window such as the menu bar cannot stand in for the app.
func frontmostApp(ctx context.Context, helper platform.Helper) (platform.WindowApp, bool, error) {
	asn, err := runLsappinfo(ctx, helper, "front")
	if err != nil || asn == "" {
		return platform.WindowApp{}, false, err
	}
	info, err := runLsappinfo(ctx, helper, "info", "-only", "bundleID", "-only", "pid", asn)
	if err != nil {
		return platform.WindowApp{}, false, err
	}
	app, ok := parseLsappinfoApp(info)
	return app, ok, nil
}

// runLsappinfo runs one Launch Services query and returns its trimmed output. A start failure or non-zero exit is a
// failed window call, so the caller reports why no app was found.
func runLsappinfo(ctx context.Context, helper platform.Helper, args ...string) (string, error) {
	result, err := helper.Run(ctx, lsappinfoPath, args, nil)
	if err != nil {
		return "", &platform.WindowError{Code: platform.WindowErrorFailed, Message: err.Error()}
	}
	if result.ExitCode != 0 {
		return "", &platform.WindowError{Code: platform.WindowErrorFailed, Message: "lsappinfo exited with status " + strconv.Itoa(result.ExitCode)}
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

// parseLsappinfoApp reads the bundle identifier and process ID from lsappinfo's quoted or bare key-value lines.
func parseLsappinfoApp(output string) (platform.WindowApp, bool) {
	bundleID := ""
	if match := lsappinfoBundleID.FindStringSubmatch(output); match != nil {
		bundleID = match[1]
	}
	pid := 0
	if match := lsappinfoPID.FindStringSubmatch(output); match != nil {
		pid, _ = strconv.Atoi(match[1])
	}
	if bundleID == "" || pid == 0 {
		return platform.WindowApp{}, false
	}
	return platform.WindowApp{PID: pid, BundleID: bundleID}, true
}

// frontmostIdentityOf is the paste identity of the app Launch Services named: its bundle ID, or "" when no app is in
// front or Nevermind (process ownPID) is, since the keyboard then belongs to the palette rather than a target.
func frontmostIdentityOf(app platform.WindowApp, found bool, ownPID int) string {
	if !found || app.PID == ownPID {
		return ""
	}
	return app.BundleID
}
