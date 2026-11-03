package backend

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// recordedStream is a chat stream as an OpenAI-compatible backend sends it: a comment line from the proxy,
// CRLF and LF line endings, a reasoning delta, a tool call split over two frames, a usage chunk with a provider
// cost, and the end marker. Every feature of the chunk format the client reads appears once.
const recordedStream = ": OPENROUTER PROCESSING\n\n" +
	"data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hel\"}}]}\r\n\r\n" +
	": keep-alive comment\n" +
	"data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning\":\"thinking\"}}]}\n\n" +
	"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"list_extensions\",\"arguments\":\"{\\\"a\"}}]}}]}\n\n" +
	"data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\":1}\"}}]}}]}\n\n" +
	"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
	"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":5,\"prompt_tokens_details\":{\"cached_tokens\":3},\"completion_tokens_details\":{\"reasoning_tokens\":2},\"cost\":0.00042}}\n\n" +
	"data: [DONE]\n\n"

// recordedEvents is what recordedStream must decode to.
var recordedEvents = []Event{
	{Type: EventContent, Text: "Hel"},
	{Type: EventReasoning, Text: "thinking"},
	{Type: EventContent, Text: "lo"},
	{Type: EventToolCallDelta, ToolCall: ToolCallDelta{Index: 0, ID: "call_1", Name: "list_extensions", ArgumentsDelta: `{"a`}},
	{Type: EventToolCallDelta, ToolCall: ToolCallDelta{Index: 0, ArgumentsDelta: `":1}`}},
	{Type: EventUsage, Usage: Usage{InputTokens: 12, OutputTokens: 5, CachedInputTokens: 3, ReasoningTokens: 2, CostUSD: 0.00042, CostKnown: true}},
	{Type: EventDone, FinishReason: "stop"},
}

// openTestStream returns a stream over body, read through readers wrapped by the caller. The client is built
// on the given port, so the same path the app uses decodes the body.
func openTestStream(t *testing.T, client *Client) *Stream {
	t.Helper()
	stream, err := client.openStream(context.Background(), requestSpec{method: "POST", path: pathChatCompletions})
	if err != nil {
		t.Fatalf("openStream: %v", err)
	}
	return stream
}

// eventsFromReader decodes everything a reader yields through the client's stream path.
func eventsFromReader(t *testing.T, reader io.Reader) []Event {
	t.Helper()
	client := streamClient(t, io.NopCloser(reader))
	return drainStream(t, openTestStream(t, client))
}

// TestRecordedStreamDecodesToExpectedEvents checks the full chunk vocabulary decodes in order.
func TestRecordedStreamDecodesToExpectedEvents(t *testing.T) {
	got := eventsFromReader(t, strings.NewReader(recordedStream))
	if !reflect.DeepEqual(got, recordedEvents) {
		t.Fatalf("events mismatch:\n got %+v\nwant %+v", got, recordedEvents)
	}
}

// TestEverySplitPointYieldsTheSameEvents delivers the recorded stream in two reads cut at every byte offset,
// and in one-byte and half-size reads. Each must decode to the same events, which proves partial lines,
// partial CRLF pairs and split data fields are joined correctly.
func TestEverySplitPointYieldsTheSameEvents(t *testing.T) {
	body := []byte(recordedStream)
	for split := 0; split <= len(body); split++ {
		reader := io.MultiReader(bytes.NewReader(body[:split]), bytes.NewReader(body[split:]))
		if got := eventsFromReader(t, reader); !reflect.DeepEqual(got, recordedEvents) {
			t.Fatalf("split at byte %d changed the events:\n got %+v\nwant %+v", split, got, recordedEvents)
		}
	}
	if got := eventsFromReader(t, iotest.OneByteReader(strings.NewReader(recordedStream))); !reflect.DeepEqual(got, recordedEvents) {
		t.Fatalf("one-byte reads changed the events:\n got %+v", got)
	}
	if got := eventsFromReader(t, iotest.HalfReader(strings.NewReader(recordedStream))); !reflect.DeepEqual(got, recordedEvents) {
		t.Fatalf("half reads changed the events:\n got %+v", got)
	}
}

// TestTruncatedStreamWithoutMarkerIsAnError checks a body that ends before [DONE] is reported as broken, and that
// an event without its blank line is discarded rather than delivered.
func TestTruncatedStreamWithoutMarkerIsAnError(t *testing.T) {
	body := `data: {"choices":[{"delta":{"content":"ok"}}]}` + "\n\n" +
		`data: {"choices":[{"delta":{"content":"lost"}}]}`
	events := eventsFromReader(t, strings.NewReader(body))
	assertTerminalError(t, events, KindBadResponse)
	if got := contentOf(events); got != "ok" {
		t.Fatalf("content = %q, want only the finished event", got)
	}
}

// TestMalformedFrameEndsTheStream checks a frame that is not JSON ends the stream with BadResponse and nothing
// after it is delivered.
func TestMalformedFrameEndsTheStream(t *testing.T) {
	body := `data: {"choices":[{"delta":{"content":"a"}}]}` + "\n\n" +
		`data: {not json` + "\n\n" +
		`data: {"choices":[{"delta":{"content":"after"}}]}` + "\n\n" + `data: [DONE]` + "\n\n"
	events := eventsFromReader(t, strings.NewReader(body))
	assertTerminalError(t, events, KindBadResponse)
	if strings.Contains(contentOf(events), "after") {
		t.Fatal("content after a malformed frame was delivered")
	}
}

// TestErrorEnvelopeInsideStreamKeepsItsKind checks a credit refusal that arrives mid-stream is classified like the
// same refusal on the HTTP status, with its numbers.
func TestErrorEnvelopeInsideStreamKeepsItsKind(t *testing.T) {
	body := `data: {"error":{"type":"insufficient_credits","message":"out","estimated_credits":9,"balance":2}}` + "\n\n"
	events := eventsFromReader(t, strings.NewReader(body))
	assertTerminalError(t, events, KindInsufficientCredits)
	if info := events[len(events)-1].Err.CreditsInfo; info == nil || info.EstimatedCredits != 9 || info.Balance != 2 {
		t.Fatalf("CreditsInfo = %+v, want 9 and 2", info)
	}
}

// TestCommentsIdsRetriesAndUnknownFieldsAreIgnored checks the lines the client does not use do not disturb
// decoding, which is what lets the backend add fields without breaking released clients.
func TestCommentsIdsRetriesAndUnknownFieldsAreIgnored(t *testing.T) {
	body := "id: 7\nretry: 100\n: comment\nevent: message\n" +
		`data: {"unknown":true,"choices":[{"delta":{"content":"ok"},"logprobs":null}],"x-extra":[1,2]}` + "\n\n" +
		"data: [DONE]\n\n"
	events := eventsFromReader(t, strings.NewReader(body))
	if got := contentOf(events); got != "ok" {
		t.Fatalf("content = %q, want ok", got)
	}
	if last := events[len(events)-1]; last.Type != EventDone {
		t.Fatalf("last event = %+v, want EventDone", last)
	}
}

// TestMultiLineDataIsJoinedWithNewlines checks data split over several data lines is one payload, as the event
// format specifies.
func TestMultiLineDataIsJoinedWithNewlines(t *testing.T) {
	body := "data: {\"choices\":\ndata: [{\"delta\":{\"content\":\"x\"}}]}\n\ndata: [DONE]\n\n"
	events := eventsFromReader(t, strings.NewReader(body))
	if got := contentOf(events); got != "x" {
		t.Fatalf("content = %q, want x", got)
	}
}

// TestLargeLineWithinTheLimitIsDelivered checks a one-megabyte content piece decodes, so the limit does not
// reject legitimate long output.
func TestLargeLineWithinTheLimitIsDelivered(t *testing.T) {
	piece := strings.Repeat("a", 1<<20)
	body := `data: {"choices":[{"delta":{"content":"` + piece + `"}}]}` + "\n\n" + "data: [DONE]\n\n"
	events := eventsFromReader(t, strings.NewReader(body))
	if got := contentOf(events); len(got) != len(piece) {
		t.Fatalf("content length = %d, want %d", len(got), len(piece))
	}
}

// TestLineOverTheLimitFailsTheStream checks a line past the 8 MiB cap ends the stream instead of growing memory.
func TestLineOverTheLimitFailsTheStream(t *testing.T) {
	body := "data: " + strings.Repeat("a", maxLineBytes+1) + "\n\n"
	events := eventsFromReader(t, strings.NewReader(body))
	assertTerminalError(t, events, KindBadResponse)
	if !strings.Contains(events[len(events)-1].Err.Message, "size limit") {
		t.Fatalf("message = %q, want the size limit", events[len(events)-1].Err.Message)
	}
}

// TestCloseIsIdempotentAndEndsTheStream checks Close can be called twice and that Next afterwards reports the end.
func TestCloseIsIdempotentAndEndsTheStream(t *testing.T) {
	client := streamClient(t, io.NopCloser(strings.NewReader(recordedStream)))
	stream := openTestStream(t, client)
	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("Next after Close = %v, want io.EOF", err)
	}
}

// TestCancellationMidStreamLeavesNoGoroutine opens a stream whose body blocks after the first event, reads that
// event, then closes the stream from the caller's side. The blocked writer must exit, and the goroutine count must
// return to its baseline. The settle loop yields the processor; it never sleeps.
func TestCancellationMidStreamLeavesNoGoroutine(t *testing.T) {
	baseline := runtime.NumGoroutine()
	reader, writer := io.Pipe()
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		_, _ = io.WriteString(writer, "data: {\"choices\":[{\"delta\":{\"content\":\"first\"}}]}\n\n")
		_, _ = io.WriteString(writer, "data: {\"choices\":[{\"delta\":{\"content\":\"never\"}}]}\n\n")
	}()
	client := streamClient(t, reader)
	stream := openTestStream(t, client)
	first, err := stream.Next()
	if err != nil || first.Type != EventContent || first.Text != "first" {
		t.Fatalf("first event = %+v, %v; want the first content piece", first, err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	<-writerDone
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("Next after cancellation = %v, want io.EOF", err)
	}
	settleGoroutines(t, baseline)
}

// settleGoroutines yields until the goroutine count drops back to baseline, failing after a bounded number of
// yields. Finished goroutines need a scheduling pass to disappear, and this waits for that without sleeping.
func settleGoroutines(t *testing.T, baseline int) {
	t.Helper()
	for range 10000 {
		if runtime.NumGoroutine() <= baseline {
			return
		}
		runtime.Gosched()
	}
	t.Fatalf("goroutines = %d, want at most the baseline %d", runtime.NumGoroutine(), baseline)
}

// TestOpenStreamReadsErrorBodiesWithoutStreaming checks a failed open returns the classified error and releases
// the connection, rather than handing back a stream.
func TestOpenStreamReadsErrorBodiesWithoutStreaming(t *testing.T) {
	port := fake.NewHTTP()
	port.Script(fake.Response{Status: 402, Body: []byte(`{"error":{"type":"insufficient_credits","message":"no","estimated_credits":4,"balance":1}}`)})
	client := newClientOver(t, port)
	stream, err := client.openStream(context.Background(), requestSpec{method: "POST", path: pathChatCompletions})
	if stream != nil || !IsKind(err, KindInsufficientCredits) {
		t.Fatalf("openStream = (%v, %v), want no stream and KindInsufficientCredits", stream, err)
	}
}
