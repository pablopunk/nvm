package manager

import "context"

type commandRunKey struct{}

// WithCommandRun marks a host-originated command invocation for diagnostics.
func WithCommandRun(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, commandRunKey{}, true)
}

// ConsumeCommandRun claims a command marker and shadows it in the returned context.
func ConsumeCommandRun(ctx context.Context) (context.Context, bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	marked, _ := ctx.Value(commandRunKey{}).(bool)
	return context.WithValue(ctx, commandRunKey{}, false), marked
}
