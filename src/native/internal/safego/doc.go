// Package safego runs goroutines, provider calls and helper work so that a panic or a hang in one
// component stays contained: the panic is recovered, reported with a support reference, and, when the
// call asks for it, restarted with backoff until a repeat offender is disabled.
//
// Entry points: Go starts a fire-and-forget goroutine, Run calls a supervised function on the calling
// goroutine and returns its outcome, and Group runs several tasks where the first failure cancels the
// rest. Options set the parent context, clock, reporter, timeout and restart policy.
//
// Layer: rank 1, beside store and logx. The package imports only platform (for Clock) and the standard
// library. Failures go to an injected Reporter, never straight to logx, so logx may import safego while
// diag, which is higher, wires logx and the recorder into a Reporter.
package safego
