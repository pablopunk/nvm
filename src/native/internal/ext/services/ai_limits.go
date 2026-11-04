package services

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Per-extension limits of the AI service. A model call, a decision and a stream each count once; a stream's pulls and
// an abort do not. The call rate is 1.x's: 30 a minute.
const (
	aiCallsPerMinute = 30
	maxOpenStreams   = 4
	maxSessions      = 8
	sessionMessages  = 40
)

// aiRateExceeded is the quota refusal of the AI call limiter, in the words 1.x uses.
func aiRateExceeded() error {
	return quota(errors.New("AI rate limit exceeded; wait a minute and try again"))
}

// aiLimitRefusal is the refusal of a call that would exceed a per-extension count.
func aiLimitRefusal(message string) error {
	return serviceError{failure: api.Failure{Code: api.CodeQuota, Message: message}, cause: api.ErrQuotaExceeded}
}
