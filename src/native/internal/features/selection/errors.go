// Package selection reads the text selected in the app the palette was summoned from, replaces it there and sends
// the undo gesture to that app. It is the one place the copy, paste and clipboard save-and-restore rules live, so
// the extension API and the AI commands share them.
//
// Layer: features. The package imports platform and the standard library.
package selection

import "errors"

// ErrSourceGone is returned when the window the selection was in has closed or changed owner.
var ErrSourceGone = errors.New("The source window is gone. Select the text and try again")

// ErrFormattedClipboard is returned when the clipboard holds HTML, RTF, files or an image that a plain-text restore
// would lose. The read refuses rather than losing the user's clipboard.
var ErrFormattedClipboard = errors.New("Cannot keep a formatted clipboard while reading selected text. Copy plain text first and try again")

// ErrNothingToUndo is returned by Undo when no replacement has been made since the last undo.
var ErrNothingToUndo = errors.New("There is no fix to undo")

// ErrSourceChanged is returned by Undo when the window in front is not the one the last replacement landed in.
var ErrSourceChanged = errors.New("The focused window changed, so nothing was undone")
