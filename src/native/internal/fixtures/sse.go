package fixtures

import (
	"encoding/json"
	"strings"
)

// FixtureReply is the assistant text the fake backend streams for every chat request.
const FixtureReply = "Hello from the fixture backend. This reply is streamed in small pieces so that a client " +
	"can test progress, cancellation and the end-of-stream marker."

// DoneFrame is the stream terminator that ends a successful completion.
const DoneFrame = "data: [DONE]\n\n"

// sseChunk is the subset of a chat completion chunk the clients read: one choice with a content delta.
type sseChunk struct {
	Object  string      `json:"object"`
	Choices []sseChoice `json:"choices"`
}

// sseChoice is one choice of a chunk.
type sseChoice struct {
	Index int      `json:"index"`
	Delta sseDelta `json:"delta"`
}

// sseDelta carries the text added by one chunk.
type sseDelta struct {
	Content string `json:"content"`
}

// StreamFrames splits text into SSE data frames of at most size bytes each, cutting only between runes.
// Every frame is a complete event, terminated by a blank line.
func StreamFrames(text string, size int) []string {
	if size <= 0 {
		size = 1
	}
	var frames []string
	var current strings.Builder
	for _, r := range text {
		if current.Len()+len(string(r)) > size && current.Len() > 0 {
			frames = append(frames, deltaFrame(current.String()))
			current.Reset()
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		frames = append(frames, deltaFrame(current.String()))
	}
	return frames
}

// MalformedStreamFrames returns the good frames for text followed by one truncated JSON frame and no
// [DONE]. A client must report the stream as broken instead of showing the partial text as complete.
func MalformedStreamFrames(text string) []string {
	frames := StreamFrames(text, 16)
	return append(frames, `data: {"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"cut sh`+"\n\n")
}

// MidStreamErrorFrames returns the good frames for text followed by an error event and no [DONE], as a
// backend emits when the upstream provider fails after the stream has started.
func MidStreamErrorFrames(text string) []string {
	frames := StreamFrames(text, 16)
	return append(frames, `data: {"error":{"type":"upstream_error","message":"Upstream connection closed mid-stream"}}`+"\n\n")
}

// deltaFrame encodes content as one SSE event carrying a chat completion chunk.
func deltaFrame(content string) string {
	payload, err := json.Marshal(sseChunk{
		Object:  "chat.completion.chunk",
		Choices: []sseChoice{{Index: 0, Delta: sseDelta{Content: content}}},
	})
	if err != nil {
		panic("fixtures: encoding a stream frame failed: " + err.Error())
	}
	return "data: " + string(payload) + "\n\n"
}
