package swifthelper

import (
	"strconv"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Subcommand is the first argument the helper expects.
const Subcommand = "window"

// PositionOnlyFlag keeps a window's size and moves only its origin.
const PositionOnlyFlag = "--position-only"

// SetFrameArgs builds `set-frame <pid> <key> <x> <y> [<width> <height>]`, or with the position-only flag
// when the write moves the window without resizing it. Numbers print the way JavaScript prints them.
func SetFrameArgs(write platform.WindowWrite) []string {
	args := []string{"set-frame", strconv.Itoa(write.PID), write.Key, formatNumber(write.Frame.X), formatNumber(write.Frame.Y)}
	if write.PositionOnly {
		return append(args, PositionOnlyFlag)
	}
	return append(args, formatNumber(write.Frame.Width), formatNumber(write.Frame.Height))
}

// SetFullScreenArgs builds `set-fullscreen <pid> <key> <true|false>`.
func SetFullScreenArgs(pid int, key string, fullScreen bool) []string {
	return []string{"set-fullscreen", strconv.Itoa(pid), key, strconv.FormatBool(fullScreen)}
}

// RaiseArgs builds `raise <pid> <key>`.
func RaiseArgs(pid int, key string) []string {
	return []string{"raise", strconv.Itoa(pid), key}
}

// ListArgs builds `list <pid>`, `focused <pid>` or a bare subcommand such as `displays`, `apps` or `all`.
func ListArgs(subcommand string, pid int) []string {
	return []string{subcommand, strconv.Itoa(pid)}
}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
