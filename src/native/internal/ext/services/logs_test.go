package services

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func logArgs(message string, data string) protocol.LogArgs {
	return protocol.LogArgs{Level: api.LogInfo, Message: message, Data: json.RawMessage(data)}
}

func TestRecentReturnsOnlyTheCallersRecords(t *testing.T) {
	r := newRig(t)
	writer := handlerFor(t, r, manifestFor("acme.writer"))
	other := handlerFor(t, r, manifestFor("acme.other"))
	mustOK(t, serve(t, writer, protocol.ServiceLogs, protocol.OpLog, logArgs("sync finished", `{"count":4}`)))
	mustOK(t, serve(t, other, protocol.ServiceLogs, protocol.OpLog, logArgs("other finished", ``)))

	var entries []api.LogEntry
	raw := mustOK(t, serve(t, writer, protocol.ServiceLogs, protocol.OpRecent, api.LogQuery{Query: "finished"}))
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(entries) != 1 || entries[0].Message != "sync finished" || entries[0].ExtensionID != "acme.writer" {
		t.Fatalf("entries = %+v; want the writer's one record", entries)
	}
	if !strings.Contains(string(entries[0].Data), `"count":4`) {
		t.Fatalf("data = %s; want the logged JSON", entries[0].Data)
	}
}

func TestRecentCannotBeSteeredToAnotherExtension(t *testing.T) {
	r := newRig(t)
	mustOK(t, serve(t, handlerFor(t, r, manifestFor("acme.other")), protocol.ServiceLogs, protocol.OpLog, logArgs("private", ``)))
	var entries []api.LogEntry
	raw := mustOK(t, serve(t, handlerFor(t, r, manifestFor("acme.spy")), protocol.ServiceLogs, protocol.OpRecent, api.LogQuery{ExtensionID: "acme.other"}))
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("a query for another extension returned %+v", entries)
	}
}

func TestLogWritesAreRateLimited(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.flood"))
	for i := 0; i < logWritesPerMinute; i++ {
		mustOK(t, serve(t, h, protocol.ServiceLogs, protocol.OpLog, logArgs("tick", ``)))
	}
	requireCode(t, serve(t, h, protocol.ServiceLogs, protocol.OpLog, logArgs("one more", ``)), api.CodeQuota, "too fast")
}

func TestOversizeLogDataIsRefused(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.one"))
	big := `"` + strings.Repeat("y", maxLogDataBytes) + `"`
	requireCode(t, serve(t, h, protocol.ServiceLogs, protocol.OpLog, logArgs("big", big)), api.CodeQuota, "KiB")
}
