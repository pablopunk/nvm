package services

import (
	"encoding/json"
	"testing"
	"testing/fstest"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundled"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// bundleHandler opens the services of an extension whose only host data is the bundle store.
func bundleHandler(t *testing.T, store *bundled.Store) *Handler {
	t.Helper()
	r := newRig(t)
	handler := New(Deps{Clock: r.clock, DataRoot: r.root, Logs: r.logs, Bundles: store}, manifestFor("acme.bundle"))
	t.Cleanup(func() { _ = handler.Close() })
	return handler
}

// readBundle asks the host for the records of one dataset and decodes them.
func readBundle(t *testing.T, h *Handler, name string) []api.BundleRecord {
	t.Helper()
	var records []api.BundleRecord
	if err := json.Unmarshal(mustOK(t, serve(t, h, protocol.ServiceBundle, protocol.OpRecords, protocol.BundleArgs{Name: name})), &records); err != nil {
		t.Fatalf("decode %q: %v", name, err)
	}
	return records
}

// TestBundleRecordsAreDecodedOnceAndKept reads a shipped dataset, then changes its file, and expects the first
// records again: the store decodes a dataset once, so a later read does not load it again.
func TestBundleRecordsAreDecodedOnceAndKept(t *testing.T) {
	files := fstest.MapFS{"words.json": {Data: []byte(`[{"id":"a","title":"Apple","keywords":["fruit"]}]`)}}
	h := bundleHandler(t, bundled.NewStore(files))
	first := readBundle(t, h, "words")
	if len(first) != 1 || first[0].Title != "Apple" || first[0].Keywords[0] != "fruit" {
		t.Fatalf("records = %+v, want the apple", first)
	}
	files["words.json"] = &fstest.MapFile{Data: []byte(`[{"id":"b","title":"Banana"}]`)}
	if again := readBundle(t, h, "words"); len(again) != 1 || again[0].Title != "Apple" {
		t.Fatalf("second read = %+v, want the first decode kept", again)
	}
}

// TestBundleNamesOutsideTheShippedDatasetsFail refuses a name that is not a dataset, including a path that would leave
// the datasets folder.
func TestBundleNamesOutsideTheShippedDatasetsFail(t *testing.T) {
	h := bundleHandler(t, bundled.NewStore(fstest.MapFS{"words.json": {Data: []byte(`[]`)}}))
	for _, name := range []string{"missing", "../words", ""} {
		if reply := serve(t, h, protocol.ServiceBundle, protocol.OpRecords, protocol.BundleArgs{Name: name}); reply.Failure == nil {
			t.Fatalf("name %q answered, want a failure", name)
		}
	}
}
