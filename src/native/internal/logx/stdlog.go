package logx

import (
	"io"
	"strings"
)

// lineWriter turns each write of the standard library's log package into one entry at a fixed level.
type lineWriter struct {
	logger *Logger
	level  Level
}

// Writer returns an io.Writer whose every write becomes one entry at level, without its trailing newline. The
// composition root points the standard library's log package at it, so the app's log.Println lines reach the file.
func (l *Logger) Writer(level Level) io.Writer {
	return lineWriter{logger: l, level: level}
}

// Write logs p as one entry and reports it fully written.
func (w lineWriter) Write(p []byte) (int, error) {
	if line := strings.TrimRight(string(p), "\n"); line != "" {
		w.logger.log(w.level, line, nil)
	}
	return len(p), nil
}
