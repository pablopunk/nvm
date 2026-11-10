// Package selection reads the text selected in the app the palette was summoned from, replaces it there and sends
// the undo gesture to that app. It is the one place the copy, paste and clipboard save-and-restore rules live, so
// the extension API and the AI commands share them.
//
// Layer: features. The package imports platform and the standard library.
package selection

import "errors"

// ErrSourceGone is returned when the window the selection was in has closed or changed owner.
//
//lint:ignore ST1005 user-facing copy, shown as written
var ErrSourceGone = errors.New("The source window is gone. Select the text and try again")

// ErrFormattedClipboard is returned when the clipboard holds HTML, RTF, files or an image that a plain-text restore
// would lose. The read refuses rather than losing the user's clipboard.
//
//lint:ignore ST1005 user-facing copy, shown as written
var ErrFormattedClipboard = errors.New("Cannot keep a formatted clipboard while reading selected text. Copy plain text first and try again")

// ErrKeyboardHeld is returned when the palette keeps the keyboard after the source window was asked for it.
//
//lint:ignore ST1005 user-facing copy, shown as written
var ErrKeyboardHeld = errors.New("The palette still has the keyboard. Select the text and try again")

// ErrNothingToUndo is returned by Undo when no replacement has been made since the last undo.
//
//lint:ignore ST1005 user-facing copy, shown as written
var ErrNothingToUndo = errors.New("There is no fix to undo")

// ErrSourceChanged is returned by Undo when the window in front is not the one the last replacement landed in.
//
//lint:ignore ST1005 user-facing copy, shown as written
var ErrSourceChanged = errors.New("The focused window changed, so nothing was undone")

// ErrConcealedContent is returned by a paste that must keep its content out of clipboard history when this clipboard
// cannot mark that content as concealed. The content is not put on the clipboard.
//
//lint:ignore ST1005 user-facing copy, shown as written
var ErrConcealedContent = errors.New("This clipboard cannot keep this content out of history, so nothing was pasted")
