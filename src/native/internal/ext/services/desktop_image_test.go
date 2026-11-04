package services

import (
	"encoding/base64"
	"strings"
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

func TestOCRInputKindsAreDetectedFromTheSource(t *testing.T) {
	png, isData, err := ocrPNGInput(api.OCRInput{Source: pngURL()})
	if err != nil || !isData || string(png) != string(pngBytes) {
		t.Fatalf("data URL input = %q, %v, %v; want the PNG bytes", png, isData, err)
	}
	if _, isData, _ := ocrPNGInput(api.OCRInput{Source: "/tmp/a.png"}); isData {
		t.Fatal("a path was read as a data URL")
	}
	if _, _, err := ocrPNGInput(api.OCRInput{Source: "data:text/plain;base64,AA=="}); err == nil || !strings.Contains(err.Error(), "PNG") {
		t.Fatalf("non-PNG data URL error = %v; want the PNG refusal", err)
	}
}
