// Package notes is the Floating Notes library: Markdown notes that autosave, floating note windows and the
// draft safety that keeps unsaved text across a crash.
//
// The behavior contract is the 1.x code in src/app/extensions/floating-notes.ts, src/app/electron/extension-host/
// extension-draft-store.ts and extension-draft-persistence.ts, and their tests, ported case for case (see the test
// files for the mapping). Entry points:
//
//   - Store holds the notes (notes.json) and the drafts (drafts.json) of one store.Store. Notes are listed newest
//     first; floating state and the last frame of a floating window are kept per note.
//   - Session edits one note. Each keystroke batch is a draft, written 300 ms after the last keystroke. The saved
//     note is written by autosave, 400 ms after the last keystroke, and the draft goes once the note is saved.
//   - A draft whose base version matches the saved note is recovered silently; one whose base differs and whose
//     text differs is a Conflict, resolved by KeepSaved or KeepDraft.
//   - Diff and Summarize compare two versions line by line for the conflict view.
//   - FitFrame moves a remembered window frame onto a display that is still connected.
//
// Layer: features (rank 4). The package imports platform, store and the standard library. It reads time only
// through the Clock port, touches the disk only through the store, and never imports search, design or MyGo.
// A read-only Store (opened without the directory's lease) refuses every write with store.ErrReadOnly.
package notes
