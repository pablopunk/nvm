package builder

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestEditFlowRequiresCompleteCurrentTurnRead(t *testing.T) {
	record := builderChat("chat-1")
	record.Extension = "user.clock"
	conversation := newFakeConversation(record)
	extensions := newFakeExtensions()
	extensions.existing["user.clock"] = clockManifestValue(t)
	extensions.files["user.clock"] = map[string][]byte{"extension.json": []byte(`{"id":"user.clock"}`), "main.go": []byte(strings.Repeat("x", apiPageBytes*2)), "helper.go": []byte("helper")}
	w := New(conversation, extensions, synchronous)
	turn := w.Toolset("chat-1")
	conversation.active["chat-1"] = turn.BuilderTurn
	read, _ := turn.Registry.Lookup(readExtensionToolName)
	write, _ := turn.Registry.Lookup(writeToolName)
	if _, err := write.Run(context.Background(), clockArgs(t)); err == nil {
		t.Fatal("unread write succeeded")
	}
	if _, err := read.Run(context.Background(), json.RawMessage(`{"id":"user.clock","offset":0}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := write.Run(context.Background(), clockArgs(t)); err == nil {
		t.Fatal("partial read authorized write")
	}
	if _, err := read.Run(context.Background(), json.RawMessage(`{"id":"user.clock","offset":99999}`)); err == nil {
		t.Fatal("skipped page accepted")
	}
	// Restart with a small complete snapshot, then change it before admission.
	extensions.files["user.clock"]["main.go"] = []byte("package main")
	if _, err := read.Run(context.Background(), json.RawMessage(`{"id":"user.clock"}`)); err != nil {
		t.Fatal(err)
	}
	extensions.files["user.clock"]["helper.go"] = []byte("changed")
	if _, err := write.Run(context.Background(), clockArgs(t)); err == nil {
		t.Fatal("stale write succeeded")
	}
	if len(extensions.installs) != 0 {
		t.Fatal("rejected write installed")
	}
	if _, err := read.Run(context.Background(), json.RawMessage(`{"id":"user.clock"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := write.Run(context.Background(), clockArgs(t)); err != nil {
		t.Fatalf("complete edit: %v", err)
	}
	next := w.Toolset("chat-1")
	conversation.active["chat-1"] = next.BuilderTurn
	oldWrite, _ := turn.Registry.Lookup(writeToolName)
	if _, err := oldWrite.Run(context.Background(), clockArgs(t)); err == nil {
		t.Fatal("old turn reserved new turn")
	}
}

type failingSnapshotExtensions struct{ *fakeExtensions }

func (e failingSnapshotExtensions) EditSnapshot(string) (map[string][]byte, any, error) {
	return nil, nil, errors.New("snapshot unavailable")
}

func TestEditFlowDoesNotTreatSnapshotErrorsAsAbsence(t *testing.T) {
	conversation := newFakeConversation(builderChat("chat-1"))
	extensions := &failingSnapshotExtensions{newFakeExtensions()}
	w := New(conversation, extensions, synchronous)
	if _, err := writeForTest(w, context.Background(), "chat-1", clockArgs(t)); err == nil || !strings.Contains(err.Error(), "snapshot unavailable") {
		t.Fatalf("write: %v", err)
	}
	if len(extensions.installs) != 0 {
		t.Fatal("snapshot error admitted write")
	}
}

func TestCancelledQueuedClaimDoesNotReserve(t *testing.T) {
	conversation := newFakeConversation(builderChat("chat-1"))
	var queued func()
	w := New(conversation, newFakeExtensions(), func(fn func()) { queued = fn })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if w.onUI(ctx, func() { t.Error("cancelled callback ran") }) {
		t.Fatal("cancelled callback succeeded")
	}
	queued()
}

type changingDuringCheck struct{ *fakeExtensions }

func (e *changingDuringCheck) Check(id string, files map[string][]byte) ([]string, error) {
	e.generation++
	return e.fakeExtensions.Check(id, files)
}

func TestEditFlowRejectsChangeDuringValidation(t *testing.T) {
	conversation := newFakeConversation(builderChat("chat-1"))
	extensions := &changingDuringCheck{newFakeExtensions()}
	w := New(conversation, extensions, synchronous)
	if _, err := writeForTest(w, context.Background(), "chat-1", clockArgs(t)); err == nil {
		t.Fatal("changed installation admitted")
	}
	if len(extensions.installs) != 0 {
		t.Fatal("stale candidate installed")
	}
}
