package api

import (
	"errors"
	"fmt"
	"strings"
)

// Command is a durable, searchable, alias-able and shortcut-able action. Its run handler is the
// extension's RunAction with ActionRef.ID equal to Command.ID.
//
// Commands and provider items stay distinct on purpose: a Command is registered once and listed
// by CommandProvider; an Item is produced per query or per view and never registered. Putting a
// global shortcut on an Item is therefore not expressible, which prevents duplicate search rows.
type Command struct {
	// ID is the local stable id. The host's global action id defaults to "extension:<extension>:<ID>".
	ID string `json:"id"`
	// ActionID is the key shortcuts and aliases bind to, instead of the default "extension:<extension>:<ID>". It must end
	// with ":" and the ID. A built-in that keeps the key 1.x stored its shortcut under sets it, as "extension-action:...".
	ActionID string   `json:"actionId,omitempty"`
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle,omitempty"`
	Aliases  []string `json:"aliases,omitempty"`
	// Keywords are more words the root search matches the command by, like aliases, for commands whose title is short.
	Keywords []string `json:"keywords,omitempty"`
	Icon     string   `json:"icon,omitempty"`
	Score    float64  `json:"score,omitempty"`
	// GlobalShortcut runs this command from anywhere, e.g. "CommandOrControl+Alt+K". A command in mode noView or
	// background runs without opening the palette and never takes focus; a view command opens the palette.
	GlobalShortcut  string         `json:"globalShortcut,omitempty"`
	Appearance      *Appearance    `json:"appearance,omitempty"`
	Mode            BackgroundMode `json:"mode,omitempty"`
	Placement       []Placement    `json:"placement,omitempty"`
	Triggers        []Trigger      `json:"triggers,omitempty"`
	DismissAfterRun bool           `json:"dismissAfterRun,omitempty"`
	// Customizable enables the shortcut and alias actions for a built-in.
	Customizable bool `json:"customizable,omitempty"`
	// Section and LearnKey are host-only presentation for built-ins, as on Item: the section and the usage-learner key.
	Section     string       `json:"section,omitempty"`
	LearnKey    string       `json:"learnKey,omitempty"`
	Actions     []Action     `json:"actions,omitempty"`
	ActionPanel *ActionPanel `json:"actionPanel,omitempty"`
}

// Validate checks the command id, its triggers and its secondary actions.
func (c Command) Validate() error {
	if c.ID == "" || c.Title == "" {
		return errors.New("command needs an id and a title")
	}
	if c.ActionID != "" && !strings.HasSuffix(c.ActionID, ":"+c.ID) {
		return fmt.Errorf("command %q: actionId %q must end with its id", c.ID, c.ActionID)
	}
	if c.Mode != "" && !oneOf(c.Mode, ModeView, ModeNoView, ModeBackground) {
		return fmt.Errorf("command %q has unknown mode %q", c.ID, c.Mode)
	}
	for _, placement := range c.Placement {
		if !oneOf(placement, PlacementSearch, PlacementRoot, PlacementHidden) {
			return fmt.Errorf("command %q has unknown placement %q", c.ID, placement)
		}
	}
	for _, trigger := range c.Triggers {
		if err := trigger.Validate(); err != nil {
			return fmt.Errorf("command %q: %w", c.ID, err)
		}
	}
	for _, action := range c.Actions {
		if err := action.Validate(); err != nil {
			return fmt.Errorf("command %q: %w", c.ID, err)
		}
	}
	return nil
}

// Invocation is the input to an ActionRun handler. The host fills only the fields that apply to
// the action that ran; everything else is zero.
type Invocation struct {
	Ref ActionRef `json:"ref"`
	// SelectedItemID is the focused item for item actions and onSelectionChange.
	SelectedItemID string `json:"selectedItemId,omitempty"`
	// Form holds submitted field values for a form's submit action.
	Form FormValues `json:"form,omitempty"`
	// EditorContent is the editor text for an editor's submit or autosave action.
	EditorContent string `json:"editorContent,omitempty"`
	// Images are the pictures a chat submit carries, in the order the user attached them.
	Images []ChatImage `json:"images,omitempty"`
	// Value is the search accessory selection for its onChange action, or the trimmed search text for onSearchChange.
	Value string `json:"value,omitempty"`
	// DraftConflict is injected only into a draft's OnConflict handler.
	DraftConflict *DraftConflict `json:"draftConflict,omitempty"`
	Launch        LaunchContext  `json:"launch"`
	// Background is set by the host when the action is declared background. Such a run may take as long as a background
	// job, because it waits on work that takes seconds, such as a microphone or a transcription.
	Background bool `json:"background,omitempty"`
}

// DraftConflict describes a stored draft whose version differs from the opened view.
type DraftConflict struct {
	Key            string `json:"key"`
	StoredVersion  string `json:"storedVersion"`
	StoredContent  string `json:"storedContent"`
	CurrentVersion string `json:"currentVersion"`
	CurrentContent string `json:"currentContent"`
}

// LaunchContext says why a handler is running. Manual palette opens leave most fields zero.
type LaunchContext struct {
	Trigger *Trigger `json:"trigger,omitempty"`
	// Event is the host event name for diagnostics, such as "files.changed".
	Event string `json:"event,omitempty"`
	// Files and ChangedPaths are bounded by the host for files.changed triggers.
	Files        []FileRef `json:"files,omitempty"`
	ChangedPaths []string  `json:"changedPaths,omitempty"`
	// Refresh is true when a host-owned view refresh re-runs the handler.
	Refresh bool `json:"refresh,omitempty"`
	// Reason is "manual", "startup", "interval", "event:<name>" or "refresh".
	Reason      string `json:"reason,omitempty"`
	StartedAtMs int64  `json:"startedAtMs"`
}

// Result is what a handler returns to the palette. The zero value means "nothing to do".
type Result struct {
	// DataChanged revalidates the surviving origin after a confirmed mutation, even if the handler returns an error.
	// Autosave revalidates the editor’s surviving parent; closed views do not refresh.
	DataChanged bool           `json:"dataChanged,omitempty"`
	View        *View          `json:"view,omitempty"`
	Next        *Action        `json:"next,omitempty"`
	Patch       *ViewPatch     `json:"patch,omitempty"`
	Navigation  NavigationMode `json:"navigation,omitempty"`
	// Toast shows a short message as a passive indicator, titled with the action that produced the result.
	Toast *Toast `json:"toast,omitempty"`
	// Draft resolves an editor draft conflict; Migrate needs replacement content.
	Draft *DraftResolution `json:"draft,omitempty"`
}

// DraftResolution answers a draft conflict. Migrate requires Content.
type DraftResolution struct {
	Key        string         `json:"key"`
	Resolution string         `json:"resolution"` // reset, restore-old or migrate
	Content    string         `json:"content,omitempty"`
	View       *View          `json:"view,omitempty"`
	Navigation NavigationMode `json:"navigation,omitempty"`
}

// Toast is a one-line message from an action result. Tone is "success", "error" or "default" (the default when empty).
// The host shows it as a passive indicator for a few seconds: errors stay four seconds, other tones 2.2 seconds.
type Toast struct {
	Message string `json:"message"`
	Tone    string `json:"tone,omitempty"`
}
