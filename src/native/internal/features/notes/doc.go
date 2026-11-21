// Package notes is the Markdown notes library behind the Floating Notes built-in: notes that the user lists, opens,
// edits and deletes, kept in one notes.json document.
//
// Entry points:
//
//   - Store holds the notes of one store.Store. Notes are listed most recently saved first. A save names the saved
//     version it was typed on top of, and a save on an older version is a conflict that writes nothing.
//
// Drafts, the unsaved text an editor keeps while the user types, belong to the extension host, not to this package.
//
// Layer: features (rank 4). The package imports platform, store and the standard library. It reads time only through
// the Clock port, touches the disk only through the store, and never imports search, design or MyGo. A read-only Store
// (opened without the directory's lease) refuses every write with store.ErrReadOnly.
package notes
