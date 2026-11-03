package backend

// Token is a bearer credential for the backend. It prints as [REDACTED] in logs, errors and fmt output, so a
// value that reaches a formatter by accident does not leak. Reveal returns the raw value for the one place
// that needs it, the Authorization header.
type Token string

// String hides the token from fmt and logs.
func (Token) String() string {
	return "[REDACTED]"
}

// GoString hides the token from %#v.
func (Token) GoString() string {
	return "backend.Token([REDACTED])"
}

// Reveal returns the raw token. Only the request builder should call it.
func (t Token) Reveal() string {
	return string(t)
}
