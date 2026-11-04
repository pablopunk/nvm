# Migrating the prototype stores onto `internal/store`

`internal/app` still persists with `loadJSON`/`saveJSON` (`app/store.go`): no envelope, no backups, no quarantine,
no lease, and a plain `.tmp` rename with the error ignored. This package replaces it. Nothing in `internal/app`
imports it yet; the package that moves each store does. This note fixes the file names, schema versions and
treatment of every prototype file so the moves do not disagree.

## Rules every migration follows

- One `store.Open` per data directory, created by the composition root (`cmd/nvm`) with the real `FS` and `Clock`
  ports and a lease from `store.AcquireLease` passed with `store.WithLease`. `Store.Close` flushes and then
  releases that lease.
- One `store.Doc` per file. Schema version starts at `1` for every new document. Do not reuse the prototype's
  struct shape as the version: the prototype wrote no version at all.
- Document names are the file names below, under the store directory. PLAN section 0 keeps that directory at
  `~/.config/nvm-native` until WP5.5 names it, which is also the prototype's directory. The first Go build
  therefore finds the prototype files there; the store quarantines them (see "Prototype files") and never
  reads them as data.
- Secret fields are never in a document. Mark a field `store:"secret"` only as a backstop; the real home is the
  Keychain port.
- Notices (`store.Notices()`) go to the palette as non-blocking banners. Do not log document content.

## Store by store

| Prototype (`app/`) | New document | Schema v1 fields | Notes |
| --- | --- | --- | --- |
| `Settings` in `settings.json` (`store.go`) | `settings.json` | `Hotkey`, `Engine`, `ClipOff`, `ClipLimit`, `Usage`, `Quicklinks`, `Disabled`, `LoginAtOpen` | Defaults in `loadSettings` (`Alt+Shift+Space`, `google`, limit 200) become the `defaults` func. Keep `Usage` as a map; the defaults func must return non-nil maps. |
| `ClipStore` in `clipboard.json` (`clipboard.go`) | `clipboard.json` | `Entries []ClipEntry` (ID, Kind, Text, Image, W, H, At, Pinned) | Image bytes live in `clipboard-images/`, which stays a separate folder. Retention deletes only through the explicit user action rules in PLAN section 4. |
| `ChatStore` in `chats.json` (`ai.go`) | `chats.json` | `Chats []*Chat` (ID, Title, Messages, Updated, Builder) | Chat messages can be large; keep the 5 MB budget in mind (the fake test covers 5 MB saves). |
| `SnippetStore` in `snippets.json` (`forms.go`) | `snippets.json` | `Items []Snippet` (ID, Name, Keyword, Text) | |
| `NoteStore` in `notes.json` (`forms.go`) | `notes.json` | `Items []Note` (ID, Text, Updated) | |
| `authInfo` in `auth.json` (`ai.go`) | none | `BaseURL`, `Email` only | `Token` moves to the Keychain port (PLAN section 4, Secrets). Do not create `auth.json` in the new app; the token is never written to any store file. |

## Schema version history

Every document starts at schema version 1 in the new app. Add a migration only when a field changes meaning or
shape; adding an optional field needs no migration, because decoding keeps defaults for missing fields.

To add version N+1 of a document declared at version N:

1. Bump the `schemaVersion` argument of `store.Doc` to N+1.
2. Append `store.Migration{From: N, Up: migrateDocVtoV(N+1)}` to the migrations, in order (one step per version
   below the current one; `Doc` panics otherwise).
3. Write a table test that calls the step directly with a stored sample (from `testdata/`) and checks the output,
   and a load test that loads a file at version N through the document.

Each step is a pure function of the stored JSON. It never touches the file: the store copies the original to
`<name>.premigrate-vN` and keeps the old version as `<name>.bak.1` when the migrated document is first saved.

## Downgrade

A build reads files written by the same schema version or an older one. A file written by a newer build
(schema version above its own) opens read-only: its fields show, nothing is overwritten, and a `NewerVersion`
notice says so. Rollback of a release candidate therefore never damages newer data, and the user keeps their
data in `<name>.json` plus the backups.

## Prototype files

The prototype files in `~/.config/nvm-native` have no envelope. The store treats them as corrupt: it moves each
one to `<name>.json.corrupt-<unix>` and starts from defaults, which keeps the bytes. The prototype is
pre-release and holds no user data worth importing, so no importer is planned. If one becomes necessary, a
migration WP reads the file itself with `os.ReadFile` before the store opens it, never through the store.

## Corruption and what the user sees

- Corrupt file: quarantined, defaults in use, `Quarantined` notice with the quarantine path. The notice copy
  needs review before the J30 journey (PLAN WP0.6, UX validation); this package only supplies the text fields.
- Newer file: read-only, `NewerVersion` notice.
- Unreadable or un-migratable file: read-only, `LoadFailed` notice. Nothing is overwritten.
- Failed save: change stays in memory, `SaveFailed` notice, retried on the next Update, Flush or Close.
- Secret field held a value: omitted from the file, `SecretOmitted` notice, `ErrSecretInStore` from Save and
  Flush. Fix the caller; do not suppress the error.
