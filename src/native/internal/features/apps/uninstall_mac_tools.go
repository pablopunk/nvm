package apps

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// macOS helpers that run through the Helper port with fixed commands. The arguments are never built from a shell
// string, and the only variable input is a checked bundle identifier or a plist path inside the app bundle.

const (
	plutilBinary      = "/usr/bin/plutil"
	mdfindBinary      = "/usr/bin/mdfind"
	plutilTimeout     = 5 * time.Second
	mdfindTimeout     = 2 * time.Second
	plutilMaxOutput   = 4096
	wrapperFolderName = "Wrapper"
)

var safeBundleQuery = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)

// PlistBundleIDReader returns a reader of CFBundleIdentifier. It runs plutil with a fixed argument list on the app's
// Info.plist, or on the single app inside a Wrapper folder when the top level has no plist (iOS apps on macOS). A
// plist that is a link is refused.
func PlistBundleIDReader(files Files, helper platform.Helper) func(appPath string) (string, error) {
	return func(appPath string) (string, error) {
		plist, err := resolvePlistPath(files, appPath)
		if err != nil {
			return "", err
		}
		ctx, cancel := context.WithTimeout(context.Background(), plutilTimeout)
		defer cancel()
		result, err := helper.Run(ctx, plutilBinary, []string{"-extract", "CFBundleIdentifier", "raw", "-o", "-", plist}, nil)
		if err != nil {
			return "", err
		}
		if result.ExitCode != 0 || result.Truncated || len(result.Stdout) > plutilMaxOutput {
			return "", errors.New("apps: plutil did not return an identifier")
		}
		return strings.TrimSpace(string(result.Stdout)), nil
	}
}

// resolvePlistPath finds the Info.plist of an app bundle without following links.
func resolvePlistPath(files Files, appPath string) (string, error) {
	direct := path.Join(appPath, "Contents", "Info.plist")
	entry, err := files.Lstat(direct)
	if err == nil {
		if entry.Symlink {
			return "", errors.New("apps: app metadata is a link")
		}
		return direct, nil
	}
	if !isMissing(err) {
		return "", err
	}
	wrapper := path.Join(appPath, wrapperFolderName)
	entries, err := files.ReadDir(wrapper)
	if err != nil {
		return "", errors.New("apps: app has no supported bundle metadata")
	}
	var inner []DirEntry
	for _, item := range entries {
		if strings.HasSuffix(item.Name, ".app") && item.Dir && !item.Symlink {
			inner = append(inner, item)
		}
	}
	if len(inner) != 1 {
		return "", errors.New("apps: app has no supported bundle metadata")
	}
	plist := path.Join(wrapper, inner[0].Name, "Info.plist")
	meta, err := files.Lstat(plist)
	if err != nil {
		return "", err
	}
	if meta.Symlink {
		return "", errors.New("apps: app metadata is outside the selected app bundle")
	}
	return plist, nil
}

// SpotlightAppFinder returns a finder of app bundles by identifier through mdfind. The identifier is checked against
// a strict pattern before it is placed in the query.
func SpotlightAppFinder(helper platform.Helper) func(bundleID string) ([]string, error) {
	return func(bundleID string) ([]string, error) {
		if !safeBundleQuery.MatchString(bundleID) {
			return nil, errors.New("apps: invalid application identifier")
		}
		query := fmt.Sprintf("(kMDItemCFBundleIdentifier == '%s'c || kMDItemCFBundleIdentifier == '%s.*'c) && kMDItemContentTypeTree == 'com.apple.application-bundle'", bundleID, bundleID)
		ctx, cancel := context.WithTimeout(context.Background(), mdfindTimeout)
		defer cancel()
		result, err := helper.Run(ctx, mdfindBinary, []string{query}, nil)
		if err != nil {
			return nil, err
		}
		return ApplicationPathsFromSpotlight(string(result.Stdout)), nil
	}
}

// ApplicationPathsFromSpotlight keeps the application bundles of a Spotlight answer: paths ending in .app that are
// not nested inside another bundle and are not in a Trash.
func ApplicationPathsFromSpotlight(output string) []string {
	var out []string
	for _, line := range strings.Split(output, "\n") {
		item := strings.TrimRight(line, "\r")
		if !strings.HasSuffix(item, ".app") || strings.Contains(item, ".app/") {
			continue
		}
		if inTrash(item) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func inTrash(p string) bool {
	for _, component := range strings.Split(p, "/") {
		if component == ".Trash" || component == ".Trashes" {
			return true
		}
	}
	return false
}
