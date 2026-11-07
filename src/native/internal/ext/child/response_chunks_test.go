package child

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func TestResponseChunksPreserveLargeJSON(t *testing.T) {
	want := protocol.Response{ID: 9, Data: json.RawMessage(`{"image":"` + strings.Repeat("a", 2<<20) + `"}`)}
	var out bytes.Buffer
	newHostLink(&out).sendResponse(want, protocol.MethodRun)
	scanner := bufio.NewScanner(&out)
	scanner.Buffer(make([]byte, 4096), protocol.MaxLineBytes)
	var assembled []byte
	sequence := 0
	last := false
	for scanner.Scan() {
		var part protocol.ResponseChunk
		if err := json.Unmarshal(scanner.Bytes(), &part); err != nil {
			t.Fatal(err)
		}
		if part.ID != want.ID || part.Sequence != sequence || last {
			t.Fatalf("invalid sequence: %+v", part)
		}
		assembled = append(assembled, part.Chunk...)
		sequence++
		last = part.Last
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !last || sequence < 2 {
		t.Fatal("missing final chunk")
	}
	var got protocol.Response
	if err := json.Unmarshal(assembled, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || !bytes.Equal(got.Data, want.Data) {
		t.Fatal("response changed")
	}
}

func TestResponseChunksRejectAggregateOverLimit(t *testing.T) {
	var out bytes.Buffer
	newHostLink(&out).sendResponse(protocol.Response{ID: 7, Data: json.RawMessage(`"` + strings.Repeat("a", protocol.MaxResponseBytes) + `"`)}, protocol.MethodRun)
	var got protocol.Response
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != 7 || got.Failure == nil {
		t.Fatal("oversized response was not rejected")
	}
}
