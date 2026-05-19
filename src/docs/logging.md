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
mise exec pnpm -- pnpm logs:tail
```

## Production

Production builds keep writing to the same bounded log file. Logs should be useful for support and self-repair, but must not include secrets, large payloads, arbitrary file contents, access tokens, or unbounded command output.

## Extension API

Extensions can write diagnostics with `ctx.logs.debug/info/warn/error(message, data?)` and inspect recent logs with `ctx.logs.recent(options)`. Reads are host-bounded and structured; extensions do not receive arbitrary filesystem access to the log file.
