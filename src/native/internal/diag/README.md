# diag

Supportability core: short error references, a bounded record of recent errors, and the glue that gives every
failure one reference across the log and the Diagnostics view. Design intent, not an API reference: the
godoc is the reference.

- **References.** `NewReference` returns `NVM-` plus five characters from a 32-symbol alphabet with no 0,
  1, I or O, drawn from the operating system's random source. The process never issues the same one twice.
- **Recorder.** Keeps the newest 200 records in memory. A record holds a reference, a code-defined component
  label, a time, redacted error text (500 characters at most), up to eight stack frames reduced to function
  and file base name, allow-listed attributes, and a count of refused attributes. Nothing else can be stored.
- **Schema guard.** `Record` is the analytics schema for errors. A test reflects over its fields, so a new
  field fails review until it is listed. Attribute values must look like identifiers, and component labels
  must be code labels; anything else is dropped or replaced, and the drop is counted.
- **Export.** `Export(ref)` gives plain text for a support request, built only from the record's own redacted
  fields.
- **Reporter.** `NewReporter(logger, recorder)` is the `safego.Reporter` the composition root installs. It
  records the failure, logs it at error level with the same reference, and returns the reference.
- **Crashes.** `InstallPanicHandler(reporter)` returns a function to defer directly in `main`. It records the
  panic, prints the reference to stderr, and panics again so the process still crashes. It installs nothing
  globally.
- **Privacy.** No prompt, clipboard, screen, file content or path enters a record. Redaction is applied to
  error text and frames, and canary tests check the record, the export and the log file.
