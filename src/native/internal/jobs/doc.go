// Package jobs runs background work on the platform clock: startup, interval and event triggers, debounced events,
// one run at a time per job, per-job budgets, backoff and halting after repeated failures, run history, and wake
// detection. Job state (enabled, halted, history) lives in one store document, so a restart keeps what the user chose.
//
// Layer: jobs. The package imports platform, store, logx, safego and the standard library. Extension triggers are
// mapped onto jobs by the subpackage extjobs; host work is declared by hostjobs. Neither is imported from here.
// Menu bar renders keep their own scheduler in internal/menubar, which applies results on the UI thread.
//
// Every run executes on its own goroutine under safego, so a job never blocks the palette and a panic is a failed
// run with a reference, not a crash. Time comes only from the clock port: tests advance a fake clock and never sleep.
package jobs
