# Moving goroutines to safego

Every long-lived or fire-and-forget goroutine in the Go app goes through `safego`, so a panic is recovered,
reported under a support reference, and a hang has a timeout. This file is the checklist for converting a
`go func() { ... }()` and lists the sites that still need converting. The conversion itself is a later work
package; WP0.7 only provides the package and the list.

## The pattern

Before:

```go
go func() {
    for range time.Tick(800 * time.Millisecond) {
        p.pollClipboard()
    }
}()
```

After:

```go
safego.Go("clipboard.watch", func(ctx context.Context) {
    ticker := clock.NewTicker(800 * time.Millisecond)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C():
            p.pollClipboard()
        }
    }
}, safego.WithContext(ctx), safego.WithClock(clock), safego.WithBackoff(5, time.Minute))
```

Rules that follow from the package:

1. **Name** every call with a stable, code-defined label such as `clipboard.watch`. Never put an extension
   id, a file name or any user text in the name: names become diagnostics components and are checked
   against a label pattern.
2. **Context**: pass the owner's context with `WithContext`. The function must return when `ctx` is done;
   `Run` and `Group` wait for it.
3. **Clock**: use the injected `platform.Clock` for tickers, sleeps and timeouts. `time.Sleep`,
   `time.Tick` and `time.After` inside the function defeat the fake clock in tests.
4. **Results**: use `safego.Run` when the caller needs the outcome (a provider call, a command). Use `Go`
   when nobody waits for the result. Use `Group` when several tasks must stop together.
5. **Restarts**: `WithBackoff(max, window)` only for long-lived watchers that should come back after a
   failure. A one-shot job should fail once and report, not retry.
6. **Timeouts**: `WithTimeout` for provider and helper calls. The function must honor its context; the
   call waits for it, so a function that ignores cancellation holds its caller.
7. **Tests**: drive time with the fake clock (`AwaitTimers`, `Advance`), never with sleeps. Check that the
   goroutine count returns to baseline, as `leak_test.go` does.
8. **Reporting**: the composition root installs `safego.SetDefaults(safego.Defaults{Clock, Reporter})` once,
   with the `diag.Reporter`. Packages never call the logger for goroutine failures themselves.

## Prototype sites to convert (`src/native/internal/app`)

| Site | What it does today | Suggested name | Notes |
| --- | --- | --- | --- |
| `clipboard.go:58` | clipboard poll every 800 ms via `time.Tick` | `clipboard.watch` | long-lived; `WithBackoff`; the clipboard WP replaces the poll with a change feed |
| `main.go:46` | sleeps 2 s, then indexes files | `files.index` | one-shot; `Go`, no restart |
| `palette.go:514` | loads installed apps in the background | `apps.load` | provider call; `Run` plus `WithTimeout`; error goes to the cache status, not a dialog |
| `palette.go:543` | waits 180 ms, then sends the paste keystroke with `xdotool` | `paste.keys` | helper process; `WithTimeout`; the sleep becomes a clock wait |
| `system.go:113` | runs an OS command and logs its error | `system.command` | helper process; `Run` plus `WithTimeout`; the command name is not part of the label |
| `system.go:178` | one-second countdown ticker for the timer command | `timer.countdown` | long-lived; cancel through the timer's stop channel mapped to `WithContext` |
| `ext.go:191` | refreshes an extension's menu bar item every 10 s | `extension.menubar` | the label is fixed; never the extension id; `WithBackoff`, because one bad extension must not stop the others |
| `ai.go:179` | streams a chat completion | `ai.stream` | already has a cancellable context; `WithContext(ctx)`; no timeout, since streams are long |

## Other production goroutines (not part of the prototype app)

These are outside `internal/app` and belong to the packages that own them. Convert them when those packages
are next changed:

- `internal/platform/real/clipboard.go:75`: `go c.pollChanges()` starts the clipboard watcher.
- `internal/platform/real/fs_watch.go:23`: `go relayWatchEvents(...)` relays file watch events.

Test doubles and tooling keep plain goroutines, because they are not product code:

- `internal/platform/fake/fs.go:268` (watch cleanup), `internal/platform/fake/clock_test.go`,
  `internal/platform/fake/http_test.go`, `internal/fixtures/backend_test.go:178`, and `internal/rigtest/*`.
