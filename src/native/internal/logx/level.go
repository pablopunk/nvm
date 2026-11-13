package logx

// Level orders log severities. The values match log/slog so a zero Options.Level means Info.
type Level int

// Levels, lowest first.
const (
	Debug Level = -4
	Info  Level = 0
	Warn  Level = 4
	Error Level = 8
)

// String returns the lowercase name written to the log file.
func (l Level) String() string {
	switch {
	case l <= Debug:
		return "debug"
	case l < Warn:
		return "info"
	case l < Error:
		return "warn"
	default:
		return "error"
	}
}
