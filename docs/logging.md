# Logging

Nevermind uses a central Electron log file for development and production diagnostics. The main process configures logging before windows and extensions are initialized, and renderer/extension diagnostics are routed back into the same file.

## Location

The canonical log file is `nevermind.log` inside Electron's logs directory (`app.getPath('logs')`). Typical locations are:

- macOS: `~/Library/Logs/Nevermind/nevermind.log`
- Linux/Windows: the platform-specific Electron logs directory

The old ad-hoc `debug.log` file is not the source of truth.

## Development

File logging is always enabled. In development, logs are also mirrored to the terminal for immediate feedback. The dev launcher writes its complete backend and Electron output to `.tmp/dev.log`, replacing the file on each start. Use this file for local incidents that cross the Astro and Electron processes.

```sh
tail -n 200 -F .tmp/dev.log
mise exec -- pnpm logs:tail
```

`logs:tail` reads the structured Electron log. Use it when only main, renderer, host, or extension events are relevant.

## Production

Production builds keep writing to the same bounded log file. Logs should be useful for support and self-repair, but must not include secrets, large payloads, arbitrary file contents, access tokens, or unbounded command output.

## Performance

Host and renderer interaction spans are written as `performance.trace` records. They use opaque trace IDs, bounded attributes, and one-way hashed identifiers so search, actions, views, jobs, AI, IPC, shortcuts, OS dispatch, and paint milestones can be compared without recording user content. Summarize local traces with `mise exec -- pnpm logs:performance [log-path]`; use p50 and p95 by operation rather than individual slow lines.

## Extension API

Extensions can write diagnostics with `ctx.logs.debug/info/warn/error(message, data?)` and inspect recent logs with `ctx.logs.recent(options)`. Reads are host-bounded and structured; extensions do not receive arbitrary filesystem access to the log file.
