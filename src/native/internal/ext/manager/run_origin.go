package manager

import "context"

type commandRunKey struct{}

type commandRunMarker struct {
	marked bool
	origin CommandOrigin
}

// WithCommandRun marks a host-originated command invocation for diagnostics.
func WithCommandRun(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return withCommandMarker(ctx, commandRunMarker{marked: true})
}

// WithCommandRunOrigin marks a host-originated command and carries its opaque source.
func WithCommandRunOrigin(ctx context.Context, origin CommandOrigin) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return withCommandMarker(ctx, commandRunMarker{marked: true, origin: origin})
}

func withCommandMarker(ctx context.Context, marker commandRunMarker) context.Context {
	return context.WithValue(ctx, commandRunKey{}, marker)
}

// ConsumeCommandRun claims a command marker and shadows it in the returned context.
func ConsumeCommandRun(ctx context.Context) (context.Context, bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	marker := markerFrom(ctx)
	return withCommandMarker(ctx, commandRunMarker{}), marker.marked
}

// ConsumeCommandOrigin claims a command marker and shadows the whole marker in the returned context.
func ConsumeCommandOrigin(ctx context.Context) (context.Context, CommandOrigin, bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	marker := markerFrom(ctx)
	return withCommandMarker(ctx, commandRunMarker{}), marker.origin, marker.marked
}

func markerFrom(ctx context.Context) commandRunMarker {
	switch value := ctx.Value(commandRunKey{}).(type) {
	case commandRunMarker:
		return value
	case bool: // Retain compatibility with contexts made by the former private representation.
		return commandRunMarker{marked: value}
	default:
		return commandRunMarker{}
	}
}
