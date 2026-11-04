package shortcuts

import "strings"

// config.json keys shortcuts by 1.x action ID, so both apps read the same file. These functions map a 2.0 action ID to
// its key and back. A 2.0 action with no 1.x equivalent keeps its own ID as its key; 1.x reads such a key as an
// unknown one and leaves it alone.
const (
	layoutKeyPrefix = "extension-action:window-management:layout."
	appKeyPrefix    = "extension-root:nevermind.apps:app:"
	windowIDPrefix  = "window."
	appIDPrefix     = "app:"
	commandIDPrefix = "command:"
)

// commandKeys are the 2.0 commands that 1.x also has, by the 2.0 ID the root command gets from its title.
var commandKeys = map[string]string{
	"command:Dictate":         "extension-root:nevermind.dictation:dictation",
	"command:Emoji & Symbols": "extension:nevermind.emoji-symbols:emoji-symbols",
}

var commandIDs = invertKeys(commandKeys)

func invertKeys(keys map[string]string) map[string]string {
	inverted := make(map[string]string, len(keys))
	for actionID, key := range keys {
		inverted[key] = actionID
	}
	return inverted
}

// PortableKeyOf returns the config.json key of a 2.0 action ID.
func PortableKeyOf(actionID string) string {
	if key, ok := commandKeys[actionID]; ok {
		return key
	}
	if rest, ok := strings.CutPrefix(actionID, windowIDPrefix); ok {
		return layoutKeyPrefix + rest
	}
	if rest, ok := strings.CutPrefix(actionID, appIDPrefix); ok {
		return appKeyPrefix + rest
	}
	return actionID
}

// ActionIDOf returns the 2.0 action ID of a config.json key, and false for a key this build does not bind.
func ActionIDOf(key string) (string, bool) {
	if actionID, ok := commandIDs[key]; ok {
		return actionID, true
	}
	if rest, ok := strings.CutPrefix(key, layoutKeyPrefix); ok {
		return windowIDPrefix + rest, true
	}
	if rest, ok := strings.CutPrefix(key, appKeyPrefix); ok {
		return appIDPrefix + rest, true
	}
	return key, strings.HasPrefix(key, commandIDPrefix)
}
