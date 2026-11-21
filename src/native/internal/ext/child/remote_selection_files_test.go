package child

import (
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func TestSelectionFilesSendsNoTarget(t *testing.T) {
	link, sent := shellLink(t, protocol.ServiceReply{Data: json.RawMessage(`["/tmp/a"]`)})
	paths, err := (remoteSelection{link: link}).Files()
	if err != nil || len(paths) != 1 || paths[0] != "/tmp/a" {
		t.Fatalf("files %v: %v", paths, err)
	}
	if sent.Service != protocol.ServiceDesktopSelection || sent.Op != protocol.OpFiles || (len(sent.Args) != 0 && string(sent.Args) != "null") {
		t.Fatalf("unexpected call: %+v", sent)
	}
}
