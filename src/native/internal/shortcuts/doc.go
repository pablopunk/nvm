// Package shortcuts is the keyboard-shortcut manager and its window-local contract. Summon owns the palette's own
// shortcut: the binding, the Linux recovery list, the retries and the banner copy. The Manager owns the other
// shortcuts and the document in shortcuts.json, which also holds the palette shortcut that Summon binds. The
// keyboard-shortcut list and recorder are palette views in the composition root. Layer: service tier (with jobs,
// windowmgmt, dictation, update and diag). It may import platform and store.
package shortcuts
