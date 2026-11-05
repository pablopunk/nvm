package shortcuts

import "strings"

// Stable IDs of the built-in commands a shortcut can name. An ID never changes when the command's title does. A command
// that 1.x also lists is stored under the 1.x key in builtInCommandKeys, so both apps share its binding.
const (
	CommandClipboardHistory   = "command:clipboard-history"
	CommandAIChats            = "command:ai-chats"
	CommandEmojiSymbols       = "command:emoji-symbols"
	CommandSearchFiles        = "command:search-files"
	CommandFindFilesByMeaning = "command:find-files-by-meaning"
	CommandQuickNotes         = "command:quick-notes"
	CommandNewFloatingNote    = "command:new-floating-note"
	CommandDictate            = "command:dictate"
	CommandDictationHistory   = "command:dictation-history"
	CommandSnippets           = "command:snippets"
	CommandExtensions         = "command:extensions"
	CommandForceQuitApps      = "command:force-quit-apps"
	CommandQuitAllApps        = "command:quit-all-apps"
	CommandKeyboardShortcuts  = "command:keyboard-shortcuts"
	CommandBackgroundTasks    = "command:background-tasks"
	CommandSettings           = "command:settings"
	CommandCheckForUpdates    = "command:check-for-updates"
	CommandShowRecentErrors   = "command:show-recent-errors"
	CommandReportProblem      = "command:report-problem"
	CommandOSPermissions      = "command:os-permissions"
	CommandFixSelectedText    = "command:fix-selected-text"
	CommandClassifyImage      = "command:classify-image"
	CommandLockScreen         = "command:lock-screen"
	CommandSleep              = "command:sleep"
	CommandRestart            = "command:restart"
	CommandSystemSettings     = "command:system-settings"
	CommandQuitNevermind      = "command:quit-nevermind"
	CommandPlaceDownloads     = "command:place-downloads"
	CommandPlaceDocuments     = "command:place-documents"
	CommandPlaceDesktop       = "command:place-desktop"
	CommandSignInOut          = "command:sign-in-out"
	CommandSwitchBackend      = "command:switch-backend"
	CommandBackendStatus      = "command:backend-status"
)

// config.json keys shortcuts by 1.x action ID, so both apps read one file. The 2.0 IDs of window layouts and apps
// translate by prefix. A 2.0 command with no 1.x equivalent keeps its own ID as its key; 1.x reads such a key as an
// unknown one and leaves it alone.
const (
	layoutKeyPrefix = "extension-action:window-management:layout."
	appKeyPrefix    = "extension-root:nevermind.apps:app:"
	windowIDPrefix  = "window."
	appIDPrefix     = "app:"
	commandIDPrefix = "command:"
)

// builtInCommandKeys maps the stable ID of each built-in command that 1.x also lists to the key 1.x stores its shortcut
// under. Commands are "extension:<extension>:<command>" and root rows "extension-root:<extension>:<row>", as 1.x builds them.
var builtInCommandKeys = map[string]string{
	CommandClipboardHistory:   "extension:nevermind.clipboard:clipboard-history",
	CommandAIChats:            "extension:nevermind.ai-builder:ai-chats",
	CommandEmojiSymbols:       "extension:nevermind.emoji-symbols:emoji-symbols",
	CommandFindFilesByMeaning: "extension:nevermind.files:find-files-by-meaning",
	CommandQuickNotes:         "extension:nevermind.floating-notes:floating-notes",
	CommandNewFloatingNote:    "extension:nevermind.floating-notes:new-floating-note",
	CommandDictate:            "extension-root:nevermind.dictation:dictation",
	CommandDictationHistory:   "extension-root:nevermind.dictation:dictation-history",
	CommandExtensions:         "extension:nevermind.extensions:extensions",
	CommandForceQuitApps:      "extension:nevermind.apps:force-quit-apps",
	CommandKeyboardShortcuts:  "extension:nevermind.shortcuts:keyboard-shortcuts",
	CommandBackgroundTasks:    "extension:nevermind.background-tasks:background-tasks",
	CommandSettings:           "extension:nevermind.settings:app-settings",
	CommandCheckForUpdates:    "extension:nevermind.updates:updates:check",
	CommandShowRecentErrors:   "extension:nevermind.diagnostics:recent-errors",
	CommandReportProblem:      "extension:nevermind.diagnostics:report-problem",
	CommandOSPermissions:      "extension-root:nevermind.permissions:os-permissions",
	CommandLockScreen:         "extension:nevermind.system:builtin:lock-screen",
	CommandSleep:              "extension:nevermind.system:builtin:sleep",
	CommandRestart:            "extension:nevermind.system:builtin:restart",
	CommandSystemSettings:     "extension:nevermind.system:builtin:settings",
	CommandQuitNevermind:      "extension:nevermind.system:builtin:quit",
	CommandPlaceDownloads:     "extension:nevermind.places:places:downloads",
	CommandPlaceDocuments:     "extension:nevermind.places:places:documents",
	CommandPlaceDesktop:       "extension:nevermind.places:places:desktop",
	CommandFixSelectedText:    "extension:nevermind.ai-commands:fix-selected-text-with-ai",
}

var builtInCommandIDs = invertKeys(builtInCommandKeys)

func invertKeys(keys map[string]string) map[string]string {
	inverted := make(map[string]string, len(keys))
	for actionID, key := range keys {
		inverted[key] = actionID
	}
	return inverted
}

// PortableKeyOf returns the config.json key of a 2.0 action ID.
func PortableKeyOf(actionID string) string {
	if key, ok := builtInCommandKeys[actionID]; ok {
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
	if actionID, ok := builtInCommandIDs[key]; ok {
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
