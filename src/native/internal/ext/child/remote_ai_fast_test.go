package child

import (
	"bufio"
	"encoding/json"
	"io"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func TestRemoteAIFastUsesAskWithFastTierAndSeparateSystemPrompt(t *testing.T) {
	reader, writer := io.Pipe()
	link := newHostLink(writer)
	got := make(chan protocol.ServiceCall, 1)
	go func() {
		line, err := bufio.NewReader(reader).ReadBytes('\n')
		if err != nil {
			return
		}
		var call protocol.ServiceCall
		if json.Unmarshal(line, &call) != nil {
			return
		}
		got <- call
		link.deliver(protocol.ServiceReply{ReplyTo: call.CallID, Data: json.RawMessage(`"corrected"`)})
	}()
	result, err := (remoteAI{link: link}).Fast("user text", "system rules")
	if err != nil || result != "corrected" {
		t.Fatalf("Fast = %q, %v", result, err)
	}
	call := <-got
	if call.Service != protocol.ServiceAI || call.Op != protocol.OpAsk {
		t.Fatalf("call = %+v; want AI ask", call)
	}
	var args protocol.AIAskArgs
	if err := json.Unmarshal(call.Args, &args); err != nil {
		t.Fatal(err)
	}
	if args.Prompt != "user text" || args.Options.System != "system rules" || args.Options.Model != api.ModelFast {
		t.Fatalf("ask args = %+v; want separate user/system and fast tier", args)
	}
}
