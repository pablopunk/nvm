package platform

import (
	"context"
	"time"
)

// ContextPaster is an optional Paster capability for callers that own a cancellable lifecycle.
// Cancellation prevents queued work and is passed through to the OS helper; it cannot recall a
// gesture already posted to the operating system.
type ContextPaster interface {
	PasteIntoFrontmostContext(ctx context.Context, expectedAppID string) error
	UndoInFrontmostContext(ctx context.Context, expectedAppID string) error
}

// Paster is the intent "paste into the app that had focus before the palette opened". The caller has
// already placed the content on the clipboard; the port sends the OS paste gesture to the frontmost app.
//
// Per-OS contract: Linux X11 sends Ctrl+V with xdotool and Linux Wayland with wtype, and either reports
// ErrUnsupported when its tool is missing; macOS sends Command+V through osascript and needs the Accessibility permission
// (UNTESTED, see real/paster_darwin.go); Windows reports ErrUnsupported until a SendInput helper exists. The
// port waits for the palette to hide before it types, through a callback the composition root supplies, so
// the keys never reach the palette. When expectedAppID is set and another app is frontmost, the port returns
// ErrConflict and types nothing, so text never lands in the wrong window.
type Paster interface {
	// PasteIntoFrontmost sends the paste gesture to the frontmost app if it is expectedAppID, or to any
	// app when expectedAppID is "".
	PasteIntoFrontmost(expectedAppID string) error
	// UndoInFrontmost sends the undo gesture (Command+Z or Ctrl+Z) to the frontmost app, with the same expectedAppID
	// rule as PasteIntoFrontmost. The caller decides what to undo; the port only sends the key.
	UndoInFrontmost(expectedAppID string) error
	// FrontmostAppID names the app that has the keyboard, in the form expectedAppID takes on this OS, so a caller
	// can remember the app it will paste into. It returns "" when the app is not known (Nevermind itself has the
	// keyboard, or no window does) and ErrUnsupported where the OS cannot name the app.
	FrontmostAppID() (string, error)
}

// Typist is an optional extension of Paster: it types text into the frontmost app as key presses, which leaves the
// clipboard alone. Only macOS implements it (capability keyboard.type-text), as the 1.x host did; a Paster without it
// refuses typing with ErrUnsupported. delay is the pause between characters, zero for none.
type Typist interface {
	TypeIntoFrontmost(expectedAppID, text string, delay time.Duration) error
}

// ContextTypist is the cancellable form of Typist.
type ContextTypist interface {
	TypeIntoFrontmostContext(ctx context.Context, expectedAppID, text string, delay time.Duration) error
}
