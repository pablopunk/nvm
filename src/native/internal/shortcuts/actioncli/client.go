package actioncli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// ErrNoInstance reports that no running instance answered.
var ErrNoInstance = errors.New("no running instance answered")

// ErrUnknownOutcome reports a request that reached the instance but whose answer was lost. It is never resent, since it
// may have run.
var ErrUnknownOutcome = errors.New("the request was sent, but its result was not received")

// Dial connects to the running instance within timeout.
type Dial func(timeout time.Duration) (net.Conn, error)

const (
	dialTimeout = time.Second
	// answerTimeout is how long the client waits for an answer: the server's dispatch bound and some slack.
	answerTimeout = DispatchTimeout + 5*time.Second
)

// Forward sends one request to the running instance and waits for its answer. A dial failure is ErrNoInstance and is
// safe to retry; any later failure is ErrUnknownOutcome and is not.
func Forward(dial Dial, now func() time.Time, actionID string) (Response, error) {
	conn, err := dial(dialTimeout)
	if err != nil {
		return Response{}, fmt.Errorf("%w: %v", ErrNoInstance, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(now().Add(answerTimeout))
	if err := json.NewEncoder(conn).Encode(Request{Version: protocolVersion, Action: actionID}); err != nil {
		return Response{}, fmt.Errorf("%w: %v", ErrUnknownOutcome, err)
	}
	if closer, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = closer.CloseWrite()
	}
	line, err := bufio.NewReader(io.LimitReader(conn, maxMessageBytes)).ReadBytes('\n')
	if (err != nil && !errors.Is(err, io.EOF)) || len(line) == 0 {
		return Response{}, fmt.Errorf("%w: %v", ErrUnknownOutcome, err)
	}
	var response Response
	if err := json.Unmarshal(line, &response); err != nil {
		return Response{}, fmt.Errorf("%w: %v", ErrUnknownOutcome, err)
	}
	return normalize(response), nil
}
