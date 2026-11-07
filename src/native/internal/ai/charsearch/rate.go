package charsearch

import "errors"

// ErrRateLimited is the error of a search that the rolling-minute limit refused. The picker shows nothing for it.
var ErrRateLimited = errors.New("AI rate limit exceeded")
