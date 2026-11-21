package child

import (
	"errors"
	"io"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func TestCallFailureWrapsTheMatchingSentinel(t *testing.T) {
	pipeReader, pipeWriter := io.Pipe()
	link := newHostLink(pipeWriter)
	go func() {
		line := make([]byte, 0, 256)
		buf := make([]byte, 1)
		for {
			if _, err := pipeReader.Read(buf); err != nil {
				return
			}
			if buf[0] == '\n' {
				break
			}
			line = append(line, buf[0])
		}
		call, ok := protocol.ParseServiceCall(line)
		if !ok || call.Service != protocol.ServiceSecrets {
			t.Errorf("call = %+v (%v); want a secrets call", call, ok)
		}
		link.deliver(protocol.ServiceReply{ReplyTo: call.CallID, Failure: &api.Failure{Code: api.CodePermission, Message: "no declaration"}})
	}()
	err := link.Call(protocol.ServiceSecrets, protocol.OpGet, protocol.KeyArgs{Key: "T"}, nil)
	if !errors.Is(err, api.ErrPermission) {
		t.Fatalf("error = %v; want the permission sentinel", err)
	}
}

func TestCloseFailsCallsThatAreWaiting(t *testing.T) {
	_, writer := io.Pipe()
	link := newHostLink(writer)
	id, reply, err := link.register()
	if err != nil || id == 0 {
		t.Fatalf("register = %d, %v", id, err)
	}
	link.close()
	if _, open := <-reply; open {
		t.Fatal("a waiting call still gets a reply after the host closed the exchange")
	}
	if _, _, err := link.register(); !errors.Is(err, api.ErrNotAvailable) {
		t.Fatalf("register after close = %v; want not available", err)
	}
}

func TestFailureErrorKeepsItsMessageAndCode(t *testing.T) {
	err := failureError(&api.Failure{Code: api.CodeQuota, Message: "too many keys"})
	if !errors.Is(err, api.ErrQuotaExceeded) || err.Error() != "too many keys: "+api.ErrQuotaExceeded.Error() {
		t.Fatalf("error = %v; want the quota sentinel with the message", err)
	}
}
