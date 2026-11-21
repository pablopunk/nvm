package backend

// EventType names what one stream event carries.
type EventType string

// The stream event types. A stream ends with exactly one terminal event, EventDone or EventError, and then
// Next returns io.EOF.
const (
	// EventContent carries a piece of the assistant's visible text in Text.
	EventContent EventType = "content"
	// EventReasoning carries a piece of the model's reasoning text in Text.
	EventReasoning EventType = "reasoning"
	// EventToolCallDelta carries a piece of a tool call in ToolCall. Pieces with the same Index belong together.
	EventToolCallDelta EventType = "tool_call_delta"
	// EventUsage carries token counts and, when the backend reports it, the provider cost in Usage.
	EventUsage EventType = "usage"
	// EventDone is the normal end of the stream. FinishReason is the last finish reason the backend sent.
	EventDone EventType = "done"
	// EventError ends the stream with Err. It covers error events inside the stream, malformed frames, a
	// stream that ends before its end marker, and a dropped connection.
	EventError EventType = "error"
)

// ToolCallDelta is one piece of a streamed tool call. The arguments arrive as JSON text in pieces.
type ToolCallDelta struct {
	// Index identifies the tool call within the message; pieces with the same Index are one call.
	Index int
	// ID is the call identifier, sent with the first piece.
	ID string
	// Name is the function name, sent with the first piece.
	Name string
	// ArgumentsDelta is the next piece of the JSON arguments text.
	ArgumentsDelta string
}

// Usage is the token count of a completed request. CostKnown reports whether the backend sent a provider cost.
type Usage struct {
	// InputTokens counts the prompt tokens.
	InputTokens int
	// OutputTokens counts the completion tokens.
	OutputTokens int
	// CachedInputTokens counts prompt tokens served from cache.
	CachedInputTokens int
	// ReasoningTokens counts the completion tokens spent on reasoning.
	ReasoningTokens int
	// CostUSD is the provider cost in US dollars, valid when CostKnown is true.
	CostUSD float64
	// CostKnown reports whether the backend sent CostUSD.
	CostKnown bool
}

// Event is one decoded item of a stream. Only the fields for its Type are set.
type Event struct {
	// Type says which field carries the payload.
	Type EventType
	// Text is the content or reasoning piece.
	Text string
	// ToolCall is the tool-call piece.
	ToolCall ToolCallDelta
	// Usage is the token count.
	Usage Usage
	// FinishReason is set on EventDone.
	FinishReason string
	// Err is set on EventError and classifies the failure.
	Err *Error
}
