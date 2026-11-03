// Package snippets is the Snippets library: named pieces of text that the user pastes often, found by name or by a
// keyword typed at the root, and pasted into the app that had focus.
//
// There is no 1.x snippets feature to port (the 1.x extensions have no snippet store), so the behavior is the
// prototype's: a name and text are required, a keyword is optional and unique, and the root finds a snippet by its
// keyword as a prefix. Placeholders are {clipboard} (the text on the clipboard when the snippet is pasted) and
// {date} (today, as 2006-01-02). Unknown braces stay as written, so text that happens to contain them is kept.
//
//   - Store holds the snippets in snippets.json of one store.Store: create, update, delete and keyword lookup.
//   - Expand fills the placeholders.
//   - Paste copies the expanded text and sends the paste gesture to the frontmost app; when the gesture is refused
//     the text stays on the clipboard and the error says why, so the user can paste it by hand.
//
// Layer: features (rank 4). The package imports platform, store and the standard library. It reads the clipboard
// and sends the paste gesture only through the Clipboard and Paster ports.
package snippets
