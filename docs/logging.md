# Logging

Nevermind uses a central Electron log file for development and production diagnostics. The main process configures logging before windows and extensions are initialized, and renderer/extension diagnostics are routed back into the same file.

## Location

The canonical log file is `nevermind.log` inside Electron's logs directory (`app.getPath('logs')`). Typical locations are:

- macOS: `~/Library/Logs/Nevermind/nevermind.log`
- Linux/Windows: the platform-specific Electron logs directory

The old ad-hoc `debug.log` file is not the source of truth.

## Development

File logging is always enabled. In development, logs are also mirrored to the terminal for immediate feedback. Prefer tailing the file when debugging cross-process issues so main, renderer, host, and extension events appear in one stream.

```sh
mise exec -- pnpm logs:tail
```

## Production

Production builds keep writing to the same bounded log file. Logs should be useful for support and self-repair, but must not include secrets, large payloads, arbitrary file contents, access tokens, or unbounded command output.

## Performance

Host and renderer interaction spans are written as `performance.trace` records. They use opaque trace IDs, bounded attributes, and one-way hashed identifiers so search, actions, views, jobs, AI, IPC, shortcuts, OS dispatch, and paint milestones can be compared without recording user content. Summarize local traces with `mise exec -- pnpm logs:performance [log-path]`; use p50 and p95 by operation rather than individual slow lines.

## Extension API

Extensions can write diagnostics with `ctx.logs.debug/info/warn/error(message, data?)` and inspect recent logs with `ctx.logs.recent(options)`. Reads are host-bounded and structured; extensions do not receive arbitrary filesystem access to the log file.
