// Package dictation is the built-in that records speech and pastes the transcript where the user is typing. Dictate
// starts a dictation and stops it, the indicator shows each state, the transcript is cleaned with the fast model when
// the settings ask, and Dictation History keeps the transcripts and the dictations whose transcription failed.
//
// Layer: ext. The package imports api, search, its cleanup and history subpackages and the standard library.
package dictation

import (
	"fmt"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/dictation/history"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity. It is the 1.x dictation extension id, so the shortcuts of its rows keep their 1.x keys.
const ID = "nevermind.dictation"

// Row ids and the shortcut actions 1.x stored their keys under. The shortcut actions are the rows' CustomizeAs: the
// host maps "command:dictate" and "command:dictation-history" to the 1.x keys, so the shortcuts keep working as in 1.x.
const (
	DictateCommandID    = "dictate"
	HistoryCommandID    = "dictation-history"
	dictateTitle        = "Dictate"
	historyTitle        = "Dictation History"
	dictateSubtitle     = "Start or stop cloud voice dictation"
	historySubtitle     = "Browse and copy previous dictations"
	settingsTitle       = "Dictation Settings"
	scoreRootCommand    = 12
	pasteTitle          = "Paste Dictation"
	copyTranscriptTitle = "Copy Transcript"
)

// Refs of the actions the views and the forms run. The args carry the saved dictation or transcript id.
const (
	RefOpenSettings     = "open-settings"
	refSaveSettings     = "save-settings"
	refRetry            = "retry-recording"
	refDeleteRecording  = "delete-recording"
	refDeleteTranscript = "delete-transcript"
	refClearHistory     = "clear-history"
	argID               = "id"
)

// Extension is the dictation built-in. Its session holds what one dictation needs while it runs, and legacy holds the
// settings and transcripts the app kept before this built-in; they answer until the first save of each.
type Extension struct {
	legacy  Legacy
	session *session
}

// Legacy is the dictation state the app held in its own documents before this built-in.
type Legacy struct {
	Settings *Settings
	History  []history.Entry
}

// session is the state of the dictation in progress. Actions run on their own goroutines, so it is guarded.
type session struct {
	mu       sync.Mutex
	starting bool
	target   string
}

// New returns the dictation built-in with the legacy state it reads until it saves its own.
func New(legacy Legacy) Extension {
	return Extension{legacy: legacy, session: &session{}}
}

// Manifest identifies the built-in and declares the capabilities its calls need, as a folder extension would.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion: "1.0",
		Identity:   api.Identity{ID: ID, Title: "Dictation"},
		Capabilities: []api.Capability{
			api.CapabilityDictation, api.CapabilityAI, api.CapabilityOCR, api.CapabilityDesktopApps,
			api.CapabilityDesktopClipboard, api.CapabilityDesktopKeyboard,
		},
		Settings:  []api.SettingDefinition{{ID: SettingDevice, Title: "Microphone", Type: api.SettingString}},
		Publishes: []string{SettingDevice},
	}
}

// Commands lists Dictate and Dictation History at the root. Their shortcuts are keyed by the command ids, which the host
// maps to the 1.x keys, so the shortcuts a user set in 1.x keep working. Loading them publishes the microphone's title
// for the Settings page's Dictation row.
func (e Extension) Commands(ctx api.Context) ([]api.Command, error) {
	publishDeviceTitle(ctx, e.readSettings(ctx))
	return []api.Command{dictateCommand(), historyCommand()}, nil
}

// dictateCommand starts or stops a dictation in the background. Its menu opens the settings.
func dictateCommand() api.Command {
	return api.Command{
		ID: DictateCommandID, Title: dictateTitle, Subtitle: dictateSubtitle, Icon: "mic", Score: scoreRootCommand,
		Aliases: []string{"dictate", "dictation", "voice dictation"}, Keywords: []string{"dictate dictation voice speech transcribe"},
		Mode: api.ModeBackground, DismissAfterRun: true, Appearance: &api.Appearance{Foreground: api.ColorNone},
		Section: search.KindCommands, LearnKey: "cmd:" + dictateTitle,
		ActionPanel: api.Panel(api.Section("", api.Run(RefOpenSettings, "Settings", nil))),
	}
}

// historyCommand opens the list of transcripts and saved dictations.
func historyCommand() api.Command {
	return api.Command{
		ID: HistoryCommandID, Title: historyTitle, Subtitle: historySubtitle, Icon: "history", Score: scoreRootCommand,
		Aliases: []string{"dictation history", "transcripts", "voice history"}, Keywords: []string{"dictation history transcripts voice history"},
		Appearance: &api.Appearance{Foreground: api.ColorNone}, Section: search.KindCommands, LearnKey: "cmd:" + historyTitle,
	}
}

// RunAction runs the rows' actions, the views' actions and the settings form's save.
func (e Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	args := in.Ref.Args
	switch in.Ref.ID {
	case DictateCommandID:
		return e.toggle(ctx)
	case HistoryCommandID:
		return e.openHistory(ctx)
	case RefOpenSettings:
		return e.openSettings(ctx)
	case refSaveSettings:
		return e.saveSettings(ctx, in.Form)
	case refRetry:
		return e.retry(ctx, args[argID])
	case refDeleteRecording:
		return e.deleteRecording(ctx, args[argID])
	case refDeleteTranscript:
		return e.deleteTranscript(ctx, args[argID])
	case refClearHistory:
		return e.clearHistory(ctx)
	}
	return api.Result{}, fmt.Errorf("the dictation built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// pushView opens view above the current page.
func pushView(ctx api.Context, view api.View) api.Result {
	return ctx.Navigation().Push(&view)
}

// replaceView shows view in place of the current page.
func replaceView(ctx api.Context, view api.View) api.Result {
	return ctx.Navigation().Replace(&view)
}
