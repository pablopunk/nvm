package protocol

import "context"

// requestMethodKey carries the extension method a request runs under.
type requestMethodKey struct{}

// WithRequestMethod records which method ctx serves, so a host service can tell a palette run from a search or a
// root listing. The supervisor sets it on every service call it answers.
func WithRequestMethod(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, requestMethodKey{}, method)
}

// RequestMethodOf returns the method WithRequestMethod recorded, or "" when none was.
func RequestMethodOf(ctx context.Context) string {
	method, _ := ctx.Value(requestMethodKey{}).(string)
	return method
}

// lifetimeKey carries the context of the runtime that serves a call. It ends when the extension stops.
type lifetimeKey struct{}

// WithLifetime records the runtime's lifetime context on ctx. A host wait, such as a confirmation, ends with it, and
// the request budget never applies to it.
func WithLifetime(ctx context.Context, lifetime context.Context) context.Context {
	return context.WithValue(ctx, lifetimeKey{}, lifetime)
}

// LifetimeOf returns the lifetime WithLifetime recorded, or a context that never ends when none was.
func LifetimeOf(ctx context.Context) context.Context {
	if lifetime, ok := ctx.Value(lifetimeKey{}).(context.Context); ok {
		return lifetime
	}
	return context.Background()
}
