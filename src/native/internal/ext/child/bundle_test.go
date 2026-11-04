package child

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestBundledRecordsCrossTheHostOnce reads one dataset three times through a built-in's context and expects the host
// to answer once: the context keeps the records, so a handler that reads them per call does not load them per call.
func TestBundledRecordsCrossTheHostOnce(t *testing.T) {
	answers := 0
	serve := func(call protocol.ServiceCall) protocol.ServiceReply {
		answers++
		if call.Service != protocol.ServiceBundle || call.Op != protocol.OpRecords {
			t.Fatalf("service %q op %q, want the bundle records", call.Service, call.Op)
		}
		return protocol.ServiceReply{Data: json.RawMessage(`[{"id":"a","title":"Apple"}]`)}
	}
	direct := NewDirectContext(api.Identity{ID: "nevermind.test", Title: "Test"}, fake.NewClock(), serve)
	c := direct.For(context.Background(), api.LaunchContext{})
	for range 3 {
		records, err := c.Data().Bundled("words").Records()
		if err != nil || len(records) != 1 || records[0].Title != "Apple" {
			t.Fatalf("records = %+v, err %v; want the apple", records, err)
		}
	}
	if answers != 1 {
		t.Fatalf("the host answered %d times, want once", answers)
	}
}
