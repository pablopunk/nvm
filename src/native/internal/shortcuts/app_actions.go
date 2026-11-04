package shortcuts

import (
	"path/filepath"
	"strings"
)

// AppActionID is the action ID of an installed app, opened by its path.
func AppActionID(path string) string {
	return appIDPrefix + path
}

// AppPathOf returns the path of the app an action ID names, and false when the action is not an app's.
func AppPathOf(actionID string) (string, bool) {
	return strings.CutPrefix(actionID, appIDPrefix)
}

// DescribeApp gives the title of an app action from its file name, for an app that is not in the app index.
func DescribeApp(actionID string) (string, bool) {
	path, ok := AppPathOf(actionID)
	if !ok {
		return "", false
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), true
}
