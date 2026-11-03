package diag

import (
	"path/filepath"
	"strings"
)

// frameLimit caps how many frames a record keeps; the top of the stack says where the failure happened.
const frameLimit = 8

// stackFrames reduces a goroutine stack, as runtime/debug prints it, to short "function file:line"
// entries. Only the function's last path element and the file's base name survive, so a home directory
// or a full import path never reaches a record. Runtime and panic plumbing frames are skipped.
func stackFrames(stack string) []string {
	lines := strings.Split(stack, "\n")
	var frames []string
	for i := 0; i+1 < len(lines) && len(frames) < frameLimit; i++ {
		function, location := lines[i], lines[i+1]
		if !isFunctionLine(function) || !strings.HasPrefix(location, "\t") {
			continue
		}
		i++
		if isPlumbingFunction(function) {
			continue
		}
		frames = append(frames, frameSummary(function, location))
	}
	return frames
}

// isFunctionLine reports whether a stack line names a function: not the goroutine header and not a
// file location, which starts with a tab.
func isFunctionLine(line string) bool {
	return line != "" && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, "goroutine ")
}

// isPlumbingFunction reports whether a frame is runtime or panic machinery rather than the failing code.
func isPlumbingFunction(function string) bool {
	return strings.HasPrefix(function, "runtime") || strings.HasPrefix(function, "panic(")
}

// frameSummary builds one "function file:line" entry from a function line and its location line.
func frameSummary(function, location string) string {
	if cut := strings.LastIndex(function, "("); cut >= 0 {
		function = function[:cut]
	}
	if slash := strings.LastIndex(function, "/"); slash >= 0 {
		function = function[slash+1:]
	}
	file := strings.TrimPrefix(location, "\t")
	if space := strings.Index(file, " +0x"); space >= 0 {
		file = file[:space]
	}
	return function + " " + filepath.Base(strings.ReplaceAll(file, `\`, "/"))
}
