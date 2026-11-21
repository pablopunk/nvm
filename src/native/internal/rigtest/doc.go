// Package rigtest drives the real Nevermind app from Go tests in the L4 rig (see scripts/rig).
//
// Helpers live behind the rig build tag, so the default build and the layering test never see them:
// `go test -tags rig ./internal/rigtest/ -run Journey` runs the journeys against an app built with
// `-tags automation`. Launch starts the binary on the current X display with an isolated HOME and
// runtime directory, PressHotkey and Type send input with xdotool, WaitFor finds elements through the
// automation plugin, and Screenshot writes PNG artifacts.
package rigtest
