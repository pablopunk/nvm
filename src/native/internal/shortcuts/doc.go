// Package shortcuts is the keyboard-shortcut manager: the canonical accelerator form and its labels, the binding
// plan with conflicts and ownership, the user's per-action choices, the palette hotkey and the recorder's key
// capture. It holds no UI: the composition root maps the Keyboard Shortcuts list and the recorder onto the palette.
//
// Layer: service tier (with jobs, windowmgmt, dictation, update and diag). It may import platform and store.
package shortcuts
