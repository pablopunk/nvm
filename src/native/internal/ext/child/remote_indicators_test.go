package child

import (
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func TestIndicatorCallsSendTheirOperations(t *testing.T) {
	link, sent := shellLink(t, protocol.ServiceReply{})
	indicators := remoteIndicators{link: link}
	if err := indicators.Show(api.IndicatorInput{ID: "copy", Title: "Copying"}); err != nil {
		t.Fatalf("Show: %v", err)
	}
	if sent.Service != protocol.ServiceIndicators || sent.Op != protocol.OpShow {
		t.Fatalf("sent %s.%s; want indicators show", sent.Service, sent.Op)
	}
	var input api.IndicatorInput
	if err := json.Unmarshal(sent.Args, &input); err != nil || input.ID != "copy" || input.Title != "Copying" {
		t.Fatalf("args = %s (%v); want the indicator input", sent.Args, err)
	}
	if err := indicators.Hide("copy"); err != nil {
		t.Fatalf("Hide: %v", err)
	}
	if sent.Op != protocol.OpHide {
		t.Fatalf("sent op %q; want hide", sent.Op)
	}
	var hide protocol.IndicatorHideArgs
	if err := json.Unmarshal(sent.Args, &hide); err != nil || hide.ID != "copy" {
		t.Fatalf("hide args = %s (%v); want the id", sent.Args, err)
	}
}
