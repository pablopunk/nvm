package shortcuts

import (
	"fmt"
	"maps"
)

// Document is what shortcuts.json holds: the palette hotkey and the user's choices. Actions maps an action to the
// accelerator the user chose for it. Disabled marks the declared shortcuts the user removed.
type Document struct {
	Palette  string            `json:"palette"`
	Actions  map[string]string `json:"actions"`
	Disabled map[string]bool   `json:"disabled"`
}

// ErrMissingShortcut is returned when the accelerator to save is blank.
var ErrMissingShortcut error = userError("Missing shortcut")

// ErrNoShortcut is returned when the action has no shortcut to remove.
var ErrNoShortcut error = userError("Shortcut not found")

// userError is an error whose text is shown to the user as written, so it starts with a capital letter.
type userError string

func (e userError) Error() string { return string(e) }

// DefaultDocument is the document of a new installation, with palette as its palette hotkey.
func DefaultDocument(palette string) Document {
	return Document{Palette: palette, Actions: map[string]string{}, Disabled: map[string]bool{}}
}

// WithAction returns the document with accelerator chosen for actionID. It fails when the accelerator is blank,
// reserved for the palette, or chosen by another action. titleOf names that action in the message.
func (d Document) WithAction(actionID, accelerator string, reserved func(string) bool, titleOf func(string) string) (Document, error) {
	accelerator = Normalize(accelerator)
	switch owner, taken := d.ownerOf(accelerator, actionID); {
	case accelerator == "":
		return d, ErrMissingShortcut
	case reserved(accelerator):
		return d, fmt.Errorf("%s is reserved for opening Nevermind", accelerator)
	case taken:
		return d, fmt.Errorf("%s is already used by %s", accelerator, nameOr(titleOf(owner)))
	}
	next := d.clone()
	next.Actions[actionID] = accelerator
	delete(next.Disabled, actionID)
	return next, nil
}

// WithoutAction returns the document with actionID's shortcut removed. A user choice is forgotten, so a declared
// shortcut of the action comes back; a declared shortcut is turned off with a marker.
func (d Document) WithoutAction(actionID string, declared bool) (Document, error) {
	next := d.clone()
	if _, chosen := d.Actions[actionID]; chosen {
		delete(next.Actions, actionID)
		return next, nil
	}
	if !declared {
		return d, ErrNoShortcut
	}
	next.Disabled[actionID] = true
	return next, nil
}

// WithPalette returns the document with accelerator as the palette hotkey. It fails when the accelerator is blank or
// is a user's action shortcut.
func (d Document) WithPalette(accelerator string, titleOf func(string) string) (Document, error) {
	accelerator = Normalize(accelerator)
	if owner, taken := d.ownerOf(accelerator, ""); accelerator == "" {
		return d, ErrMissingShortcut
	} else if taken {
		return d, fmt.Errorf("%s is already used by %s", accelerator, nameOr(titleOf(owner)))
	}
	next := d.clone()
	next.Palette = accelerator
	return next, nil
}

// ownerOf finds the action, other than except, that holds accelerator.
func (d Document) ownerOf(accelerator, except string) (string, bool) {
	for actionID, held := range d.Actions {
		if actionID != except && held == accelerator {
			return actionID, true
		}
	}
	return "", false
}

func (d Document) clone() Document {
	next := d
	next.Actions = maps.Clone(d.Actions)
	next.Disabled = maps.Clone(d.Disabled)
	if next.Actions == nil {
		next.Actions = map[string]string{}
	}
	if next.Disabled == nil {
		next.Disabled = map[string]bool{}
	}
	return next
}

func nameOr(title string) string {
	if title == "" {
		return "another action"
	}
	return title
}
