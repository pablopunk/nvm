package backend

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// maxFuzzEvents bounds how many events one fuzz input may produce, so a pathological input cannot hang a run.
const maxFuzzEvents = 1 << 16

// FuzzSSE feeds arbitrary bytes through the stream decoder. The property is that it never panics, always ends
// with a terminal event or io.EOF, and never emits an event after its terminal one. Under plain go test only the
// seed corpus runs; go test -fuzz explores beyond it.
func FuzzSSE(f *testing.F) {
	f.Add([]byte(recordedStream))
	f.Add([]byte("data: [DONE]\n\n"))
	f.Add([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"cut sh\n\n"))
	f.Add([]byte("data: {\"error\":{\"type\":\"upstream_error\",\"message\":\"x\"}}\n\n"))
	f.Add([]byte(": comment only\n\n"))
	f.Add([]byte("data:\r\n\r\ndata:\r\r\n"))
	f.Add([]byte("event: x\ndata: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":-1}]}}]}\n\ndata: [DONE]\n\n"))
	f.Add([]byte(strings.Repeat("data: {\"choices\":[]}\n", 64)))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, body []byte) {
		stream := &Stream{
			client: newClientOver(t, readerHTTP{}),
			caller: context.Background(),
			ex:     exchange{status: 200, body: io.NopCloser(strings.NewReader(string(body))), release: func() {}},
			frames: newFrameReader(strings.NewReader(string(body))),
		}
		defer func() { _ = stream.Close() }()
		terminated := false
		for range maxFuzzEvents {
			event, err := stream.Next()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				t.Fatalf("Next returned a Go error %v; failures must arrive as EventError", err)
			}
			if terminated {
				t.Fatalf("event %+v arrived after a terminal event", event)
			}
			if event.Type == EventDone || event.Type == EventError {
				terminated = true
			}
		}
		t.Fatalf("stream produced more than %d events", maxFuzzEvents)
	})
}
