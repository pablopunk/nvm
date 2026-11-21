package logx

import (
	"io"
	"maps"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Options configure a logger. Zero values take the defaults noted on each field.
type Options struct {
	// FileName is the active file inside the log directory. Default "nvm.log".
	FileName string
	// Level is the lowest level kept. Default Info.
	Level Level
	// MaxBytes is the size at which the active file rotates. Default 1 MiB.
	MaxBytes int64
	// MaxFiles counts every file kept, the active one included. Default 5.
	MaxFiles int
	// FlushInterval bounds how long an entry waits in memory before it reaches the file. Default 1s.
	FlushInterval time.Duration
	// Stderr, when set, also receives every kept entry as one JSON line.
	Stderr io.Writer
	// Redactor scrubs entries before they are stored or written. Default NewRedactor().
	Redactor *Redactor
}

const (
	defaultFileName      = "nvm.log"
	defaultMaxBytes      = 1 << 20
	defaultMaxFiles      = 5
	defaultFlushInterval = time.Second
)

// withDefaults fills every zero field with its default.
func (o Options) withDefaults() Options {
	if o.FileName == "" {
		o.FileName = defaultFileName
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = defaultMaxBytes
	}
	if o.MaxFiles <= 0 {
		o.MaxFiles = defaultMaxFiles
	}
	if o.FlushInterval <= 0 {
		o.FlushInterval = defaultFlushInterval
	}
	if o.Redactor == nil {
		o.Redactor = NewRedactor()
	}
	return o
}

// Logger writes entries at or above its level. Loggers made with With share the file, the ring and the
// flusher of the logger they came from, and differ only in the fields they add.
type Logger struct {
	core   *core
	fields map[string]any
}

// New opens a logger that writes to dir on files and paces its flushes on clock. It creates the directory
// and continues the active file. A nil files keeps entries in memory and on stderr only. The caller must
// Close the logger to stop the flusher and write what is pending.
func New(files platform.FS, clock platform.Clock, dir string, opts Options) (*Logger, error) {
	if clock == nil {
		panic("logx: New needs a clock")
	}
	o := opts.withDefaults()
	c := newCore(clock.Now, files, dir, o)
	if files != nil {
		if err := files.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
		if err := c.load(); err != nil {
			return nil, err
		}
		c.startFlusher(clock, o.FlushInterval)
	}
	return &Logger{core: c}, nil
}

// With returns a child logger that adds fields to every entry it writes. Field values are redacted once,
// here, so each entry only pays for its own fields.
func (l *Logger) With(fields F) *Logger {
	merged := make(map[string]any, len(l.fields)+len(fields))
	maps.Copy(merged, l.fields)
	maps.Copy(merged, l.core.redactor.Fields(fields))
	return &Logger{core: l.core, fields: merged}
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, fields F) { l.log(Debug, msg, fields) }

// Info logs at info level.
func (l *Logger) Info(msg string, fields F) { l.log(Info, msg, fields) }

// Warn logs at warn level.
func (l *Logger) Warn(msg string, fields F) { l.log(Warn, msg, fields) }

// Error logs at error level. Error entries reach the file immediately instead of on the next tick.
func (l *Logger) Error(msg string, fields F) { l.log(Error, msg, fields) }

// Recent returns up to limit of the newest entries, oldest first; a limit of zero or less returns every
// retained entry. The result is a copy, so callers cannot change the ring.
func (l *Logger) Recent(limit int) []Entry {
	l.core.mu.Lock()
	defer l.core.mu.Unlock()
	return l.core.ring.newest(limit)
}

// Flush writes the pending entries to the file now.
func (l *Logger) Flush() error {
	return l.core.flush()
}

// Close stops the flusher, writes the pending entries and returns the error of that final write. Entries
// logged after Close stay in memory and reach the file only through a later Flush of an error entry.
func (l *Logger) Close() error {
	l.core.stop()
	return l.core.flush()
}

// Path returns the active file's path, or "" for a logger without a file.
func (l *Logger) Path() string {
	if l.core.files == nil {
		return ""
	}
	return l.core.activePath
}

// Dropped returns how many entries were lost to encoding or write failures since the logger opened.
func (l *Logger) Dropped() int {
	l.core.mu.Lock()
	defer l.core.mu.Unlock()
	return l.core.dropped
}

// log redacts the message and fields, stamps the entry and hands it to the core.
func (l *Logger) log(level Level, msg string, fields F) {
	c := l.core
	if level < c.level {
		return
	}
	merged := make(map[string]any, len(l.fields)+len(fields))
	maps.Copy(merged, l.fields)
	maps.Copy(merged, c.redactor.Fields(fields))
	c.write(Entry{
		Time:    c.now(),
		Level:   level,
		Message: c.redactor.Text(msg),
		Fields:  merged,
	})
}
