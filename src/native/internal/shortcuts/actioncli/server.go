package actioncli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"time"
)

// Dispatcher runs one action by ID and reports what happened. It is called once per request, on a goroutine of the
// server; it must honor ctx, whose deadline is the server's DispatchTimeout.
type Dispatcher func(ctx context.Context, actionID string) Response

const (
	// readTimeout bounds how long a client may take to send its request.
	readTimeout = 2 * time.Second
	// DispatchTimeout bounds a dispatch, readiness wait included, so a client never waits forever.
	DispatchTimeout = 20 * time.Second
)

// Serve answers requests on ln until it is closed or ctx ends. Each connection carries one request and one response;
// now sets the connection deadlines.
func Serve(ctx context.Context, ln net.Listener, now func() time.Time, dispatch Dispatcher) {
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return
			}
			continue
		}
		go serveOne(ctx, conn, now, dispatch)
	}
}

func serveOne(ctx context.Context, conn net.Conn, now func() time.Time, dispatch Dispatcher) {
	defer conn.Close()
	_ = conn.SetReadDeadline(now().Add(readTimeout))
	response := answer(ctx, conn, dispatch)
	_ = conn.SetWriteDeadline(now().Add(readTimeout))
	_ = json.NewEncoder(conn).Encode(response)
}

// answer reads one request and dispatches it. A malformed request is rejected before any lookup.
func answer(ctx context.Context, conn io.Reader, dispatch Dispatcher) Response {
	line, err := bufio.NewReader(io.LimitReader(conn, maxMessageBytes)).ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return Response{Outcome: Rejected, Message: "the request was not received"}
	}
	var request Request
	if json.Unmarshal(line, &request) != nil || request.Version != protocolVersion {
		return Response{Outcome: Rejected, Message: "the request is not one this version understands"}
	}
	if err := ValidateActionID(request.Action); err != nil {
		return Response{Outcome: Rejected, Message: err.Error()}
	}
	dispatchCtx, cancel := context.WithTimeout(ctx, DispatchTimeout)
	defer cancel()
	return normalize(dispatch(dispatchCtx, request.Action))
}

// normalize turns an answer with an unknown outcome into a failure, so a client never reads success from it.
func normalize(r Response) Response {
	switch r.Outcome {
	case Completed, Started, Opened, Rejected, Failed:
		return r
	}
	return Response{Outcome: Failed, Message: "the action gave no result"}
}
