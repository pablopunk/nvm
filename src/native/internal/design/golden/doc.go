// Package golden is the visual regression harness. It renders a view headlessly with ui.NewTester at a
// given size, scale and appearance, and compares the frame with a checked-in PNG under
// testdata/golden/<group>/<name>.<theme>.<scale>x.png.
//
// Views must be deterministic: fixtures only, no randomness, no network, no wall clock. The tester's
// clock is fake: it stands still until Advance, and what depends on time (animations, Context.After,
// Context.Now) follows it, so a golden does not depend on when the test runs.
//
// Updating goldens is one command, run in the package directory: go test -run '^TestName$' -update .
// (or go test . -update for every test of the package). A mismatch writes testdata/golden/_failures/
// with the actual frame and a diff image, red where pixels differ.
//
// Layer: design (test tooling). It imports MyGo's ui, which the platform grep test allows here only.
package golden
