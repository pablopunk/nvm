package logx

import (
	"log"
	"strings"
	"testing"
)

// TestStandardLogLinesBecomeEntries routes the standard library's log package through Writer, as the app does, and
// checks each line lands as one redacted entry at the level asked for.
func TestStandardLogLinesBecomeEntries(t *testing.T) {
	logger, _, _ := newTestLogger(t, Options{})
	std := log.New(logger.Writer(Warn), "", 0)
	std.Println("chat: could not send:", "token="+canary)
	entries := logger.Recent(0)
	if len(entries) != 1 || entries[0].Level != Warn || !strings.HasPrefix(entries[0].Message, "chat: could not send:") {
		t.Fatalf("entries = %+v, want one warn line", entries)
	}
	if strings.Contains(entries[0].Message, canary) {
		t.Fatalf("message carries the secret: %q", entries[0].Message)
	}
}
