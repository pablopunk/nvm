package backend

import (
	"encoding/json"
)

// doneMarker is the data payload that ends a successful stream.
const doneMarker = "[DONE]"

// wireChunk is the subset of a chat completion chunk the client reads. Unknown fields are ignored, so the
// backend can add fields without breaking released clients.
type wireChunk struct {
	Choices []wireChoice `json:"choices"`
	Usage   *wireUsage   `json:"usage"`
	Error   *envelope    `json:"error"`
}

// wireChoice is one choice of a chunk.
type wireChoice struct {
	Delta        wireDelta `json:"delta"`
	FinishReason string    `json:"finish_reason"`
}

// wireDelta carries what one chunk adds to the message.
type wireDelta struct {
	Content          string         `json:"content"`
	Reasoning        string         `json:"reasoning"`
	ReasoningContent string         `json:"reasoning_content"`
	ToolCalls        []wireToolCall `json:"tool_calls"`
}

// wireToolCall is one tool-call piece inside a delta.
type wireToolCall struct {
	Index    int              `json:"index"`
	ID       string           `json:"id"`
	Function wireToolFunction `json:"function"`
}

// wireToolFunction holds the function name and a piece of its arguments.
type wireToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// wireUsage is the usage object the backend sends in the final chunk.
type wireUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	PromptDetails    struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
	Cost *float64 `json:"cost"`
}

// decodeChunk turns one data payload into events. It returns the finish reason the payload carried, if any,
// and an error when the payload is malformed or is an error envelope. An error means the stream must end.
func decodeChunk(payload string) ([]Event, string, *Error) {
	var chunk wireChunk
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return nil, "", &Error{Kind: KindBadResponse, Status: 200, Message: "stream frame is not valid JSON", Cause: err}
	}
	if chunk.Error != nil {
		return nil, "", errorFromEnvelope(0, *chunk.Error, "")
	}
	var events []Event
	finish := ""
	for _, choice := range chunk.Choices {
		events = appendDeltaEvents(events, choice.Delta)
		if choice.FinishReason != "" {
			finish = choice.FinishReason
		}
	}
	if chunk.Usage != nil {
		events = append(events, Event{Type: EventUsage, Usage: usageFromWire(*chunk.Usage)})
	}
	return events, finish, nil
}

// appendDeltaEvents adds the content, reasoning and tool-call pieces of one delta, in that order.
func appendDeltaEvents(events []Event, delta wireDelta) []Event {
	if delta.Content != "" {
		events = append(events, Event{Type: EventContent, Text: delta.Content})
	}
	reasoning := delta.Reasoning
	if reasoning == "" {
		reasoning = delta.ReasoningContent
	}
	if reasoning != "" {
		events = append(events, Event{Type: EventReasoning, Text: reasoning})
	}
	for _, call := range delta.ToolCalls {
		events = append(events, Event{Type: EventToolCallDelta, ToolCall: ToolCallDelta{
			Index:          call.Index,
			ID:             call.ID,
			Name:           call.Function.Name,
			ArgumentsDelta: call.Function.Arguments,
		}})
	}
	return events
}

// usageFromWire converts the backend usage object to Usage.
func usageFromWire(wire wireUsage) Usage {
	usage := Usage{
		InputTokens:       wire.PromptTokens,
		OutputTokens:      wire.CompletionTokens,
		CachedInputTokens: wire.PromptDetails.CachedTokens,
		ReasoningTokens:   wire.CompletionDetails.ReasoningTokens,
	}
	if wire.Cost != nil {
		usage.CostUSD = *wire.Cost
		usage.CostKnown = true
	}
	return usage
}
