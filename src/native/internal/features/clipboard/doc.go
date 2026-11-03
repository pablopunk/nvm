// Package clipboard is the clipboard history library: what the copy watcher captures, how entries are
// stored, searched and retained, the image store and its garbage collection, the copy and paste actions,
// and the row and action data the palette renders.
//
// The behavior contract is the 1.x code in src/app/electron/clipboard/*.ts and its tests, ported case for
// case (see the test files for the mapping). Entry points:
//
//   - History holds entries newest first, enforces retention and persists through Persistence.
//   - Watcher reads the system clipboard (change notification when the platform offers it, adaptive
//     polling otherwise) and applies the capture rules: concealed content is dropped before anything is
//     read, startup content is ignored, our own writes are suppressed, sizes are capped.
//   - ImageStore saves PNG files under one folder with 0600 mode and collects the ones no entry references.
//   - Actions copies and pastes an entry through the Clipboard and Paster ports.
//   - View builds rows and removal actions as plain data; the palette decides how to draw them.
//
// Layer: features (rank 4). The package imports platform, store, logx, safego and the standard library. It
// reads time only through the Clock port, touches the OS only through the Clipboard, Paster and FS ports, and
// never imports search, design or MyGo. A concealed entry is refused by History and never reaches storage,
// logs or change events.
package clipboard
