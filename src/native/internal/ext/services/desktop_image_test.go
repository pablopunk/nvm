package services

import (
	"encoding/base64"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

var pngBytes = []byte("\x89PNG\r\n\x1a\nfake")

func pngURL() string {
	return pngDataURLPrefix + base64.StdEncoding.EncodeToString(pngBytes)
}

func TestClipboardImageRoundTripsAsAPNGDataURL(t *testing.T) {
	host, deps := newDesktopHost(t)
	h := desktopHandler(t, deps, api.CapabilityDesktopClipboard)
	mustOK(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpWriteImage, protocol.WriteImageArgs{DataURL: pngURL()}))
	var image protocol.ImageResult
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpReadImage, nil)), &image); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !image.Found || image.DataURL != pngURL() {
		t.Fatalf("read back %+v; want the PNG data URL that was written", image)
	}
	if got, _ := host.clipboard.ReadImagePNG(); string(got) != string(pngBytes) {
		t.Fatalf("clipboard holds %q", got)
	}
}

func TestClipboardImageWriteRefusesNonPNGAndConcealed(t *testing.T) {
	_, deps := newDesktopHost(t)
	h := desktopHandler(t, deps, api.CapabilityDesktopClipboard)
	requireCode(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpWriteImage, protocol.WriteImageArgs{DataURL: "data:image/jpeg;base64,AA=="}), api.CodeInvalid, "PNG")
	requireCode(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpWriteImage, protocol.WriteImageArgs{DataURL: pngURL(), Concealed: true}), api.CodeUnavailable, "history")
}
