package learning

import (
	"strings"
	"testing"
)

func TestTraceDropsSensitiveAndBoundsInputs(t *testing.T) {
	traces := NewTraces()
	for _, text := range []string{"password is hunter2", "see https://example.test/?token=abc", "read /tmp/private", "```secret```", strings.Repeat("x", 1001),
		"use AKIAABCDEFGHIJKLMNOP for the deploy", "the key is sk-live-0123456789abcdef", "paste 9f8e7d6c5b4a39281706f5e4d3c2b1a0 there"} {
		traces.Message("chat", "user", text)
	}
	traces.Outcome("chat", "success")
	snapshot, ok := traces.Snapshot("chat")
	if !ok || len(snapshot.Messages) != 0 || len(snapshot.ToolOutcomes) != 1 {
		t.Fatalf("unsafe trace retained: %+v", snapshot)
	}
	for i := 0; i < 25; i++ {
		traces.Message("chat", "user", "Prefer clear labels")
	}
	snapshot, _ = traces.Snapshot("chat")
	if len(snapshot.Messages) != 20 {
		t.Fatalf("messages: %d", len(snapshot.Messages))
	}
	traces.Clear()
	if _, ok := traces.Snapshot("chat"); ok {
		t.Fatal("clear retained trace")
	}
}

func TestTraceDropsUnfencedSource(t *testing.T) {
	traces := NewTraces()
	traces.Message("chat", "assistant", "package main\nfunc Run() {}")
	traces.Outcome("chat", "success")
	snapshot, _ := traces.Snapshot("chat")
	if len(snapshot.Messages) != 0 {
		t.Fatal("source text was retained")
	}
}
