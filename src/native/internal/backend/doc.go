// Package backend is the typed client for the Nevermind backend: chat streaming, models and credits, the AI
// decision endpoint, the compatibility manifest, and device sign-in. It is the only place backend URLs are
// built, and every request carries the compatibility headers and the bearer token from the injected TokenSource.
//
// Layer: backend, above logx and platform. It imports only those and the standard library. It never reads the
// wall clock, never stores tokens, and never logs them. Callers persist tokens in the keychain themselves.
//
// Errors are *Error values with a Kind; show UserMessage to people. Streams yield events and end with a
// terminal event, so a caller handles one sequence whether the failure was in the stream or on the network.
package backend
