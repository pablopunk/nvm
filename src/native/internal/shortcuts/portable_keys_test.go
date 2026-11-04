package shortcuts

import (
	"strings"
	"testing"
)

var allBuiltInCommandIDs = []string{
	CommandClipboardHistory, CommandAIChats, CommandEmojiSymbols, CommandSearchFiles, CommandFindFilesByMeaning,
	CommandQuickNotes, CommandNewFloatingNote, CommandDictate, CommandDictationHistory, CommandSnippets, CommandExtensions, CommandForceQuitApps,
	CommandQuitAllApps, CommandKeyboardShortcuts, CommandBackgroundTasks, CommandSettings, CommandCheckForUpdates,
	CommandShowRecentErrors, CommandReportProblem, CommandOSPermissions, CommandFixSelectedText,
	CommandTriageSelectedText, CommandClassifyImage, CommandLockScreen, CommandSleep, CommandRestart,
	CommandSystemSettings, CommandQuitNevermind, CommandPlaceDownloads, CommandPlaceDocuments, CommandPlaceDesktop,
	CommandSignInOut, CommandSwitchBackend, CommandBackendStatus,
}

func TestBuiltInCommandIDsAreStableNamesWithOneKeyEach(t *testing.T) {
	seen := map[string]bool{}
	for _, id := range allBuiltInCommandIDs {
		if !strings.HasPrefix(id, "command:") || strings.ContainsAny(id, " &") {
			t.Errorf("%q is not a stable command ID", id)
		}
		if seen[id] {
			t.Errorf("%q is listed twice", id)
		}
		seen[id] = true
	}
	for id, key := range builtInCommandKeys {
		if !seen[id] {
			t.Errorf("table entry %q is not a command ID", id)
		}
		if !strings.HasPrefix(key, "extension:") && !strings.HasPrefix(key, "extension-root:") {
			t.Errorf("%q maps to %q, which is not a 1.x command or root key", id, key)
		}
	}
	if len(builtInCommandIDs) != len(builtInCommandKeys) {
		t.Errorf("two commands share a 1.x key: %d keys for %d commands", len(builtInCommandIDs), len(builtInCommandKeys))
	}
}

func TestEveryCommandIDRoundTripsThroughItsConfigKey(t *testing.T) {
	for _, id := range allBuiltInCommandIDs {
		key := PortableKeyOf(id)
		if back, bound := ActionIDOf(key); !bound || back != id {
			t.Errorf("%s -> %q -> %q, %v", id, key, back, bound)
		}
	}
}

func TestSharedCommandsUseTheOneXKeyString(t *testing.T) {
	want := map[string]string{
		CommandDictate:         "extension-root:nevermind.dictation:dictation",
		CommandEmojiSymbols:    "extension:nevermind.emoji-symbols:emoji-symbols",
		CommandSettings:        "extension:nevermind.settings:app-settings",
		CommandCheckForUpdates: "extension:nevermind.updates:updates:check",
		CommandOSPermissions:   "extension-root:nevermind.permissions:os-permissions",
	}
	for id, key := range want {
		if got := PortableKeyOf(id); got != key {
			t.Errorf("PortableKeyOf(%s) = %q; want %q", id, got, key)
		}
	}
}

func TestCommandsWithoutAOneXEquivalentKeepTheirOwnKey(t *testing.T) {
	for _, id := range []string{CommandSnippets, CommandSearchFiles, CommandQuitAllApps, CommandFixSelectedText} {
		if _, shared := builtInCommandKeys[id]; shared {
			t.Errorf("%s has a 1.x key; the test expects it to be 2.0 only", id)
		}
		if key := PortableKeyOf(id); key != id {
			t.Errorf("PortableKeyOf(%s) = %q; want its own ID", id, key)
		}
	}
}
