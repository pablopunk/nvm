# logx

Structured, redacted logging for the Go app. Design intent, not an API reference: the godoc is the reference.

- **Shape.** One JSON object per line: `time`, `level`, `msg`, and `fields` (only when present). Levels are
  debug, info, warn and error. Error entries reach the file at once; other entries wait for the flush tick
  (one second by default) or a full batch.
- **Files.** The active file is `nvm.log` in the log directory, written through the platform FS port with
  a temporary file and a rename. When the next batch would pass `MaxBytes` (1 MiB), the files shift and
  at most `MaxFiles` (5, active file included) remain.
- **Memory.** The newest 1000 entries stay in a ring for the Diagnostics view. `Recent` returns copies.
- **Redaction.** Every message and field passes through `Redactor` before it is stored or written. Keys
  such as token, secret, password, authorization, cookie and key lose their values. Free text loses
  bearer tokens, provider key prefixes, URL query and fragment values, `key=value` pairs and long random
  runs. Ordinary prose and plain paths survive. Bare secrets with no key, prefix or URL cannot be detected;
  code must never log them.
- **Ownership.** The composition root creates the logger with `New` and calls `SetDefault` once. Libraries
  take a `*Logger`. `Default` exists for code that cannot be handed one, and returns a stderr-only fallback
  until then.
- **Tests.** Canary strings (a token in a field, a message, an error, a URL query and a nested header) must
  never appear in the file or the ring. Rotation, reopening, concurrent writers and the flush tick run on
  the fake FS and clock.
