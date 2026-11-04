package services

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func TestStorageRoundTripAndDelete(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.one"))
	mustOK(t, serve(t, h, protocol.ServiceStorage, protocol.OpSet, protocol.KeyValueArgs{Key: "city", Value: []byte(`{"name":"Lisbon"}`)}))
	got := decodeGet(t, mustOK(t, serve(t, h, protocol.ServiceStorage, protocol.OpGet, protocol.KeyArgs{Key: "city"})))
	if !got.Found || string(got.Value) != `{"name":"Lisbon"}` {
		t.Fatalf("get = %+v; want the stored JSON", got)
	}
	mustOK(t, serve(t, h, protocol.ServiceStorage, protocol.OpDelete, protocol.KeyArgs{Key: "city"}))
	if decodeGet(t, mustOK(t, serve(t, h, protocol.ServiceStorage, protocol.OpGet, protocol.KeyArgs{Key: "city"}))).Found {
		t.Fatal("a deleted key is still found")
	}
}

func TestStorageKeyCountQuotaAllowsOverwriteAtTheLimit(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.one"))
	for i := 0; i < maxStorageKeys; i++ {
		mustOK(t, serve(t, h, protocol.ServiceStorage, protocol.OpSet, protocol.KeyValueArgs{Key: keyName(i), Value: []byte(`1`)}))
	}
	requireCode(t, serve(t, h, protocol.ServiceStorage, protocol.OpSet, protocol.KeyValueArgs{Key: "one-too-many", Value: []byte(`1`)}), api.CodeQuota, "storage keys")
	mustOK(t, serve(t, h, protocol.ServiceStorage, protocol.OpSet, protocol.KeyValueArgs{Key: keyName(0), Value: []byte(`2`)}))
}

func TestStorageRefusesAnOversizeValueAndKeepsTheOldOne(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.one"))
	mustOK(t, serve(t, h, protocol.ServiceStorage, protocol.OpSet, protocol.KeyValueArgs{Key: "doc", Value: []byte(`"small"`)}))
	big := `"` + strings.Repeat("x", maxStorageValueBytes) + `"`
	requireCode(t, serve(t, h, protocol.ServiceStorage, protocol.OpSet, protocol.KeyValueArgs{Key: "doc", Value: []byte(big)}), api.CodeQuota, "")
	got := decodeGet(t, mustOK(t, serve(t, h, protocol.ServiceStorage, protocol.OpGet, protocol.KeyArgs{Key: "doc"})))
	if string(got.Value) != `"small"` {
		t.Fatalf("refused write changed the value to %d bytes", len(got.Value))
	}
}

func TestStorageRejectsEmptyKeys(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.one"))
	requireCode(t, serve(t, h, protocol.ServiceStorage, protocol.OpSet, protocol.KeyValueArgs{Key: "", Value: []byte(`1`)}), api.CodeInvalid, "")
}

func TestStorageIsNamespacedPerExtension(t *testing.T) {
	r := newRig(t)
	writer := handlerFor(t, r, manifestFor("acme.writer"))
	reader := handlerFor(t, r, manifestFor("acme.reader"))
	mustOK(t, serve(t, writer, protocol.ServiceStorage, protocol.OpSet, protocol.KeyValueArgs{Key: "shared", Value: []byte(`"mine"`)}))
	if decodeGet(t, mustOK(t, serve(t, reader, protocol.ServiceStorage, protocol.OpGet, protocol.KeyArgs{Key: "shared"}))).Found {
		t.Fatal("another extension read a key it did not write")
	}
}

func TestStorageSurvivesReopen(t *testing.T) {
	r := newRig(t)
	first := handlerFor(t, r, manifestFor("acme.one"))
	mustOK(t, serve(t, first, protocol.ServiceStorage, protocol.OpSet, protocol.KeyValueArgs{Key: "kept", Value: []byte(`42`)}))
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	second := handlerFor(t, r, manifestFor("acme.one"))
	got := decodeGet(t, mustOK(t, serve(t, second, protocol.ServiceStorage, protocol.OpGet, protocol.KeyArgs{Key: "kept"})))
	if !got.Found || string(got.Value) != "42" {
		t.Fatalf("after reopen get = %+v; want 42", got)
	}
}

func keyName(i int) string {
	return fmt.Sprintf("key-%04d", i)
}
