package chat

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
)

// NoticeKind names the situation a notice explains. The palette picks its action from it.
type NoticeKind string

// The notice kinds. Each one has its own title and copy; the copy never includes server text.
const (
	NoticeSignIn           NoticeKind = "sign_in"
	NoticeCredits          NoticeKind = "credits"
	NoticeSpendCap         NoticeKind = "spend_cap"
	NoticeRateLimited      NoticeKind = "rate_limited"
	NoticeTooLarge         NoticeKind = "too_large"
	NoticeRequestShape     NoticeKind = "request_shape"
	NoticeModelUnavailable NoticeKind = "model_unavailable"
	NoticeUpdate           NoticeKind = "update"
	NoticeOffline          NoticeKind = "offline"
	NoticeTimeout          NoticeKind = "timeout"
	NoticeCutOff           NoticeKind = "cut_off"
	NoticeStepLimit        NoticeKind = "step_limit"
	NoticeFailed           NoticeKind = "failed"
	NoticeNote             NoticeKind = "note"
)

// Notice is a line in the transcript that explains why a turn did not finish, or why it could not start.
type Notice struct {
	Kind    NoticeKind `json:"kind"`
	Title   string     `json:"title"`
	Message string     `json:"message"`
}

// Sentinel errors the service returns for refusals it makes itself, before or instead of a backend call.
var (
	ErrEmpty            = errors.New("chat: the message is empty")
	ErrBusy             = errors.New("chat: this chat is still answering")
	ErrNotFound         = errors.New("chat: no such chat")
	ErrNothingToAsk     = errors.New("chat: there is no message to ask again")
	ErrNotSignedIn      = errors.New("chat: not signed in")
	ErrRouteUnsupported = errors.New("chat: the active model speaks a wire format this version cannot read")
	ErrNothingToDecide  = errors.New("chat: no tool call is waiting for an answer")
)

// Copy for the notices the service writes. Each line says what the user can do next.
const (
	copySignIn           = "Sign in to Nevermind in Account to chat with AI."
	copyRouteUnsupported = "This AI model needs a newer Nevermind. Update Nevermind to keep using AI."
	copyCutOff           = "The reply stopped before it finished. Ask again to get a new reply."
	copyTooLarge         = "This chat is too long for one request. Start a new chat to keep going."
	copyFailed           = "The reply failed. Ask again, or try later."
	copyEmptyReply       = "The model sent no text. Ask again."
	copyStepLimit        = "The reply used every step without a final answer. Ask again to keep going."
	copyTurnDeadline     = "The reply ran past 10 minutes, so it was stopped. Ask again to get a new reply."
	copyPricing          = "Nevermind cannot price the selected AI model right now, so this request was not sent. Try again later or choose another model."
)

// SignInNotice is the notice shown when a turn starts without a connected account.
func SignInNotice() Notice {
	return Notice{Kind: NoticeSignIn, Title: "Sign in to use AI", Message: copySignIn}
}

// NoticeFor explains err, the error that ended a turn. partial is the reply text that arrived before the failure;
// a broken reply with text is reported as cut off, so the user knows the text they saw is incomplete.
func NoticeFor(err error, partial string) Notice {
	if errors.Is(err, ErrNotSignedIn) {
		return SignInNotice()
	}
	if errors.Is(err, ErrEmptyReply) {
		return Notice{Kind: NoticeFailed, Title: "No reply", Message: copyEmptyReply}
	}
	if errors.Is(err, agent.ErrTurnDeadline) {
		return Notice{Kind: NoticeTimeout, Title: "Reply took too long", Message: copyTurnDeadline}
	}
	if errors.Is(err, agent.ErrStepLimit) {
		return Notice{Kind: NoticeStepLimit, Title: "Step limit reached", Message: copyStepLimit}
	}
	if errors.Is(err, ErrRouteUnsupported) {
		return Notice{Kind: NoticeUpdate, Title: "Update Nevermind", Message: copyRouteUnsupported}
	}
	if spendCap, ok := auth.SpendCapNoticeFromError(err); ok {
		return Notice{Kind: NoticeSpendCap, Title: spendCap.Title, Message: spendCap.Message}
	}
	var typed *backend.Error
	if !errors.As(err, &typed) {
		return Notice{Kind: NoticeFailed, Title: "Reply failed", Message: copyFailed}
	}
	if notice, ok := noticeForCode(typed, partial); ok {
		return notice
	}
	return noticeForKind(typed, partial)
}

// noticeForCode maps the server's machine-readable error types that need their own copy.
func noticeForCode(typed *backend.Error, partial string) (Notice, bool) {
	switch typed.Code {
	case "prompt_too_large", "request_too_large":
		return Notice{Kind: NoticeTooLarge, Title: "Chat too long", Message: copyTooLarge}, true
	case "unsupported_request_shape":
		return Notice{Kind: NoticeRequestShape, Title: "Request not supported", Message: backend.UserMessage(typed)}, true
	case "pricing_unavailable":
		return Notice{Kind: NoticeModelUnavailable, Title: "AI model unavailable", Message: copyPricing}, true
	case "model_not_configured":
		return Notice{Kind: NoticeModelUnavailable, Title: "Model unavailable", Message: backend.UserMessage(typed)}, true
	}
	return Notice{}, false
}

// noticeForKind maps a failure kind. A broken stream with text already shown is reported as a cut-off reply.
func noticeForKind(typed *backend.Error, partial string) Notice {
	switch typed.Kind {
	case backend.KindUnauthorized:
		return Notice{Kind: NoticeSignIn, Title: "Sign in again", Message: backend.UserMessage(typed)}
	case backend.KindInsufficientCredits:
		return Notice{Kind: NoticeCredits, Title: "Credits needed", Message: backend.UserMessage(typed)}
	case backend.KindRateLimited:
		return Notice{Kind: NoticeRateLimited, Title: "Too many requests", Message: backend.UserMessage(typed)}
	case backend.KindIncompatible:
		return Notice{Kind: NoticeUpdate, Title: "Update Nevermind", Message: backend.UserMessage(typed)}
	case backend.KindOffline:
		return Notice{Kind: NoticeOffline, Title: "You're offline", Message: backend.UserMessage(typed)}
	case backend.KindTimeout:
		return Notice{Kind: NoticeTimeout, Title: "No answer", Message: backend.UserMessage(typed)}
	}
	if partial != "" {
		return Notice{Kind: NoticeCutOff, Title: "Reply cut off", Message: copyCutOff}
	}
	return Notice{Kind: NoticeFailed, Title: "Reply failed", Message: backend.UserMessage(typed)}
}
