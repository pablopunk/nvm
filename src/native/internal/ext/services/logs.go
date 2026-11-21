package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// Log limits: a record's message and data are clipped in size, and an extension may write a bounded number of
// records per minute, so a flooding extension cannot fill the log file or push out other records.
const (
	maxLogMessageBytes = 2 << 10
	maxLogDataBytes    = 4 << 10
	defaultLogRecent   = 100
	maxLogRecent       = 500
)

func (h *Handler) serveLogs(call protocol.ServiceCall) (any, error) {
	switch call.Op {
	case protocol.OpLog:
		return nil, h.writeLog(call.Args)
	case protocol.OpRecent:
		return h.recentLogs(call.Args)
	}
	return nil, invalid("logs has no operation %q", call.Op)
}

func (h *Handler) writeLog(args json.RawMessage) error {
	var request protocol.LogArgs
	if err := decodeArgs(args, &request); err != nil {
		return err
	}
	if !h.logWrites.allow() {
		return quota(errors.New("the extension writes records too fast; this record was dropped"))
	}
	if len(request.Message) > maxLogMessageBytes {
		request.Message = request.Message[:maxLogMessageBytes]
	}
	if len(request.Data) > maxLogDataBytes {
		return quota(fmt.Errorf("log data may be at most %d KiB", maxLogDataBytes>>10))
	}
	fields := logx.F{}
	if len(request.Data) > 0 {
		var data any
		if err := json.Unmarshal(request.Data, &data); err != nil {
			return invalid("log data must be JSON")
		}
		fields["data"] = data
	}
	h.emit(request.Level, request.Message, fields)
	return nil
}

func (h *Handler) emit(level api.LogLevel, message string, fields logx.F) {
	switch level {
	case api.LogDebug:
		h.logger.Debug(message, fields)
	case api.LogWarn:
		h.logger.Warn(message, fields)
	case api.LogError:
		h.logger.Error(message, fields)
	default:
		h.logger.Info(message, fields)
	}
}

// recentLogs returns this extension's records only. The owner is the extension field the host wrote, never a
// field the query could set, so one extension cannot read another's records.
func (h *Handler) recentLogs(args json.RawMessage) ([]api.LogEntry, error) {
	var query api.LogQuery
	if err := decodeArgs(args, &query); err != nil {
		return nil, err
	}
	limit := query.Limit
	if limit <= 0 {
		limit = defaultLogRecent
	}
	limit = min(limit, maxLogRecent)
	var matched []api.LogEntry
	for _, entry := range h.deps.logsOrDefault().Recent(0) {
		if entry.Fields["extension"] != h.manifest.ID || entry.Fields["source"] != "extension" {
			continue
		}
		view := logEntryView(entry, h.manifest.ID)
		if matchesQuery(view, query) {
			matched = append(matched, view)
		}
	}
	if len(matched) > limit {
		matched = matched[len(matched)-limit:]
	}
	return matched, nil
}

func (d Deps) logsOrDefault() *logx.Logger {
	if d.Logs != nil {
		return d.Logs
	}
	return logx.Default()
}

func logEntryView(entry logx.Entry, extensionID string) api.LogEntry {
	view := api.LogEntry{
		TimestampMs: entry.Time.UnixMilli(),
		Level:       api.LogLevel(entry.Level.String()),
		Source:      "extension",
		ExtensionID: extensionID,
		Message:     entry.Message,
	}
	if scope, ok := entry.Fields["scope"].(string); ok {
		view.Scope = scope
	}
	if data, ok := entry.Fields["data"]; ok {
		view.Data, _ = json.Marshal(data)
	}
	return view
}

func matchesQuery(entry api.LogEntry, query api.LogQuery) bool {
	if query.Level != "" && entry.Level != query.Level {
		return false
	}
	if query.SinceMs > 0 && entry.TimestampMs < query.SinceMs {
		return false
	}
	if query.Query != "" && !strings.Contains(strings.ToLower(entry.Message), strings.ToLower(query.Query)) {
		return false
	}
	return true
}
