//go:build race

package store

// raceEnabled reports whether the race detector instruments this build. Timing budgets are not meaningful
// under instrumentation, so budget tests check them only in plain builds.
const raceEnabled = true
