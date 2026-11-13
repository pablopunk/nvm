# safego

Runs goroutines, provider calls and helper work so that one failing component cannot take the app down.
Design intent, not an API reference: the godoc is the reference. Conversion steps and the list of sites
still to move are in [MIGRATING.md](MIGRATING.md).

- **Entry points.** `Go` starts a named goroutine. `Run` calls a supervised function and returns its
  outcome. `Group` runs tasks together: the first failure cancels the rest, and `Wait` returns that error.
- **Panics.** A recovered panic becomes a `*PanicError` with its stack. It is reported once through the
  installed `Reporter`, which returns a support reference that the error carries.
- **Restarts.** `WithBackoff(max, window)` restarts after a panic, an error or a timeout, waiting 250 ms,
  500 ms, and so on up to 30 s. More than `max` failures inside `window` disable the call, and the last
  report says so.
- **Timeouts.** `WithTimeout` cancels the attempt's context. The function must return when it is cancelled.
- **Clock and defaults.** Timeouts and backoff run on the injected clock, so tests advance time instead of
  sleeping. `SetDefaults` installs the clock and reporter once for the process; without a clock, timeouts
  and backoff panic at the call site rather than fail silently.
- **Watch.** `Watching().Live(name)` counts running goroutines by name, for leak tests and diagnostics.
- **Layering.** Failures go to a `Reporter`, not to logx, so logx can use safego for its flusher while the
  diag package wires logx and the recorder together.
