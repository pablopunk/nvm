package fixtures

// Routes the fake backend serves.
const (
	ChatCompletionsPath          = "/api/v1/chat/completions"
	ActiveModelPath              = "/api/v1/active-model"
	MePath                       = "/api/me"
	TranscriptionsPath           = "/api/v1/audio/transcriptions"
	TranscriptionPreparationPath = "/api/v1/audio/transcription-preparations"
)

// RetryAfterSeconds is the Retry-After value sent with ModeRateLimited.
const RetryAfterSeconds = "30"

// Exact response bodies. Tests compare them byte for byte, so a change here is a change of the contract.

// UnauthorizedBody is the plain-text body the backend returns for a rejected token.
const UnauthorizedBody = "Unauthorized"

// InsufficientCreditsBody is the 402 body: the request would exceed the remaining balance. Its shape matches
// the backend's insufficient_credits error, with estimated_credits 95 and balance 81.
const InsufficientCreditsBody = `{"error":{"type":"insufficient_credits","message":"Request cost would exceed remaining balance","estimated_credits":95,"balance":81}}`

// RateLimitedBody is the 429 body; the same wait is in the Retry-After header.
const RateLimitedBody = `{"error":{"type":"rate_limited","message":"Rate limit exceeded (chat)","retry_after":30}}`

// ServerErrorBody is the 500 body.
const ServerErrorBody = `{"error":{"type":"internal_error","message":"Internal server error"}}`

// InvalidRequestBody is the 400 body for a chat request without messages.
const InvalidRequestBody = `{"error":{"type":"invalid_request","message":"messages is required"}}`

// NotFoundBody is the 404 body for routes the fake does not serve.
const NotFoundBody = `{"error":{"type":"not_found","message":"No such route"}}`

// MethodNotAllowedBody is the 405 body for a known route called with the wrong method.
const MethodNotAllowedBody = `{"error":{"type":"method_not_allowed","message":"Method not allowed"}}`

// ActiveModelBody is the descriptor of the fixture route for every model tier: an openai-completions route with
// credits left and no notice. Its names are fixtures, not product model IDs.
const ActiveModelBody = `{"id":"fixture-route","name":"Fixture Route","contextWindow":128000,"maxInputTokens":100000,"maxTokens":4096,"reasoning":false,"thinkingLevel":"off","input":["text"],"api":"openai-completions","provider":"fixture","baseUrl":"https://fixture.test","modelTier":"free","creditKind":"free","credits":{"paid":0,"free":81,"total":81},"notice":"ok","noticeReason":"","costEstimate":null}`

// MeBody is the signed-in account. The email uses the reserved example.test domain.
const MeBody = `{"id":"user_fixture","email":"fixture@example.test","plan":"free","credits":{"balance":81}}`

// TranscriptionBody is the answer of the transcription route for any recording.
const TranscriptionBody = `{"text":"Fixture transcript of the recording."}`

// OperationBody is the 200 answer of the transcription preparation route and of an approval.
const OperationBody = `{"ok":true}`

// InvalidDeviceCodeBody is the 400 answer to an approval of a code that is unknown, expired, used or approved.
const InvalidDeviceCodeBody = "Invalid or expired code"
