package logx

import (
	"os"
	"sync"
	"time"
)

var (
	defaultMu     sync.RWMutex
	defaultLogger *Logger
	// fallbackLogger serves Default before the composition root installs a logger. It has no file and
	// stamps entries with zero times, so output produced that early is still visible on stderr.
	fallbackLogger = &Logger{core: newCore(
		func() time.Time { return time.Time{} },
		nil,
		"",
		Options{Stderr: os.Stderr}.withDefaults(),
	)}
)

// SetDefault installs the process logger returned by Default. Only the composition root calls it, once,
// right after New; library code receives a *Logger instead.
func SetDefault(logger *Logger) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultLogger = logger
}

// Default returns the process logger installed with SetDefault, or the stderr fallback before that.
func Default() *Logger {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	if defaultLogger != nil {
		return defaultLogger
	}
	return fallbackLogger
}
