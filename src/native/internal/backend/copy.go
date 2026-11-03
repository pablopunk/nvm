package backend

import (
	"errors"
	"fmt"
	"time"
)

// Palette copy for every failure the client reports. It is kept in this one file so copy review is one read.
// Each line is active voice and says what the user can do. Lines with a verb that takes a number are format
// strings; the call site fills them in.
const (
	copyOffline             = "You're offline. Nevermind will reconnect when your network is back."
	copyTimeout             = "Nevermind didn't answer in time. Check your connection and try again."
	copyCanceled            = "Cancelled."
	copyUnauthorized        = "Your Nevermind sign-in has expired. Sign in again to keep using AI."
	copyCreditsDetailed     = "This request needs about %d credits and your balance is %d. Add credits on your dashboard, then try again."
	copyCreditsGeneric      = "You're out of credits for this request. Add credits on your dashboard, then try again."
	copyRateLimitedDetailed = "Too many requests. Try again in %d seconds."
	copyRateLimitedGeneric  = "Too many requests. Wait a moment and try again."
	copyServer              = "Nevermind's servers are having trouble. Try again in a few minutes."
	copyFreeSpendPaused     = "Free AI is paused until capacity returns. Try again later, or add credits to keep going."
	copyModelUnavailable    = "No AI model is available right now. Try again in a few minutes."
	copyIncompatible        = "This version of Nevermind is no longer supported. Update Nevermind to keep using it."
	copyIncompatibleAt      = "This version of Nevermind is no longer supported. Update Nevermind to %s or newer to keep using it."
	copyRequestShape        = "This model can't handle that request. Remove attachments or web search, or choose another model."
	copyRequestTooLarge     = "That request is too large. Shorten the text or remove attachments, then try again."
	copyDecisionRequest     = "Nevermind couldn't read that decision request. Check the questions, then try again."
	copyDecisionAnswer      = "The decision model returned an answer Nevermind couldn't use. Try again."
	copyRejected            = "Nevermind couldn't complete that request. Try again, and update Nevermind if it keeps failing."
	copyBadResponse         = "Nevermind received a response it doesn't understand. Update Nevermind, then try again."
	copySignInExpired       = "The sign-in code expired before it was approved. Start sign-in again."
	copySignInDenied        = "The sign-in was refused. Start sign-in again if that was a mistake."
	copyUnknown             = "Something went wrong. Try again."
)

// UserMessage returns the palette copy for err. Errors without a backend classification get a generic line, so
// a raw transport or decoding error never reaches the user.
func UserMessage(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrSignInExpired) {
		return copySignInExpired
	}
	if errors.Is(err, ErrSignInDenied) {
		return copySignInDenied
	}
	typed := asError(err)
	if typed == nil {
		return copyUnknown
	}
	if message, ok := messageForCode(typed.Code); ok {
		return message
	}
	return messageForKind(typed)
}

// messageForCode returns the copy for a server error type that the user can act on differently from its kind.
func messageForCode(code string) (string, bool) {
	switch code {
	case codeFreeSpendCapReached:
		return copyFreeSpendPaused, true
	case codeModelNotConfigured, codePricingUnavailable, codeDecisionModelUnavailable:
		return copyModelUnavailable, true
	case codeUnsupportedRequestShape:
		return copyRequestShape, true
	case codeRequestTooLarge:
		return copyRequestTooLarge, true
	case codeInvalidDecisionRequest:
		return copyDecisionRequest, true
	case codeDecisionProviderInvalid:
		return copyDecisionAnswer, true
	default:
		return "", false
	}
}

// messageForKind returns the copy for a failure kind, using the details the error carries when they help.
func messageForKind(err *Error) string {
	switch err.Kind {
	case KindOffline:
		return copyOffline
	case KindTimeout:
		return copyTimeout
	case KindCanceled:
		return copyCanceled
	case KindUnauthorized:
		return copyUnauthorized
	case KindInsufficientCredits:
		return creditsMessage(err.CreditsInfo)
	case KindRateLimited:
		return rateLimitMessage(err.RetryAfter)
	case KindServer:
		return copyServer
	case KindIncompatible:
		return incompatibleMessage(err.MinimumVersion)
	case KindRejected:
		return copyRejected
	case KindBadResponse:
		return copyBadResponse
	default:
		return copyUnknown
	}
}

// creditsMessage names the cost and balance when the backend sent them.
func creditsMessage(info *CreditsInfo) string {
	if info == nil || info.EstimatedCredits <= 0 {
		return copyCreditsGeneric
	}
	return fmt.Sprintf(copyCreditsDetailed, info.EstimatedCredits, info.Balance)
}

// rateLimitMessage gives the wait when the backend asked for one, rounded up to whole seconds.
func rateLimitMessage(wait time.Duration) string {
	if wait <= 0 {
		return copyRateLimitedGeneric
	}
	seconds := int((wait + time.Second - 1) / time.Second)
	return fmt.Sprintf(copyRateLimitedDetailed, seconds)
}

// incompatibleMessage names the minimum version when the backend sent one, so the user knows what to install.
func incompatibleMessage(minimum string) string {
	if minimum == "" {
		return copyIncompatible
	}
	return fmt.Sprintf(copyIncompatibleAt, minimum)
}
