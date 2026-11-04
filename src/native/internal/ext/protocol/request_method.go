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
