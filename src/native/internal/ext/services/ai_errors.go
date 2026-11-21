package services

import (
	"context"
	"errors"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ai/extai"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// copyNotSignedIn is the palette line of an AI call made while signed out.
const copyNotSignedIn = "Sign in to Nevermind to use AI."

// aiFailure maps the error of a model call to the failure the extension sees. Input the host refuses keeps 1.x's
// message; backend failures use the palette copy of the backend client.
func aiFailure(err error) error {
	var input extai.InputError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &input):
		return serviceError{failure: api.Failure{Code: api.CodeInvalid, Message: string(input)}, cause: api.ErrInvalidInput}
	case errors.Is(err, extai.ErrTimedOut):
		return serviceError{failure: api.Failure{Code: api.CodeTimeout, Message: extai.ErrTimedOut.Error()}, cause: api.ErrNotAvailable}
	case errors.Is(err, chat.ErrNotSignedIn):
		return unavailable("%s", copyNotSignedIn)
	case errors.Is(err, chat.ErrRouteUnsupported):
		return unavailable("%s", chat.NoticeFor(err, "").Message)
	case errors.Is(err, api.ErrNotAvailable):
		return unavailable("%s", err.Error())
	}
	var typed *backend.Error
	if errors.As(err, &typed) {
		return backendFailure(typed)
	}
	if errors.Is(err, context.Canceled) {
		return serviceError{failure: api.Failure{Code: api.CodeCancelled, Message: "Cancelled."}}
	}
	return err
}

// backendFailure maps a backend refusal or transport failure to its failure code.
func backendFailure(typed *backend.Error) error {
	failure := api.Failure{Message: backend.UserMessage(typed)}
	cause := api.ErrNotAvailable
	switch typed.Kind {
	case backend.KindTimeout:
		failure.Code = api.CodeTimeout
	case backend.KindCanceled:
		failure.Code = api.CodeCancelled
		cause = nil
	case backend.KindRateLimited, backend.KindInsufficientCredits:
		failure.Code, cause = api.CodeQuota, api.ErrQuotaExceeded
	case backend.KindRejected:
		failure.Code, cause = api.CodeInvalid, api.ErrInvalidInput
	case backend.KindBadResponse:
		failure.Code, cause = api.CodeInternal, nil
	default:
		failure.Code = api.CodeUnavailable
	}
	return serviceError{failure: failure, cause: cause}
}
