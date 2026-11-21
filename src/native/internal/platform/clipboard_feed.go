package platform

import "context"

// ClipboardCopy is one copy delivered with its content by a ClipboardFeed. Exactly one of Text, PNG and Files is
// set, unless Concealed is, in which case none is and the copy must not be recorded.
type ClipboardCopy struct {
	Text string
	PNG  []byte
	// Files are the absolute paths of files copied in a file manager.
	Files []string
	// Concealed marks a copy a password manager asked clipboard tools not to record. Its content is never read.
	Concealed bool
	// Initial marks the content that was already on the clipboard when the feed started. It is a baseline, not a
	// copy the user made while the app ran.
	Initial bool
}

// ClipboardFeed is an optional extension of Clipboard for systems where a client cannot read the clipboard unless
// it has focus (Wayland). The OS pushes every copy with its content, even while the app is in the background,
// so clipboard history sees each copy of a burst instead of whatever is on the clipboard when it next reads.
type ClipboardFeed interface {
	// WatchCopies starts delivering copies until ctx is done, then closes the channel. A nil channel with a nil
	// error means this system needs no feed and the ordinary reads work. A nil channel with an error means the
	// feed is needed but cannot run (the watch tool is missing); callers log it once and keep polling.
	WatchCopies(ctx context.Context) (<-chan ClipboardCopy, error)
}
