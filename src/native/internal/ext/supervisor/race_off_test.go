//go:build !race

package supervisor

// raceEnabled is false when the test binary runs without the race detector.
const raceEnabled = false
