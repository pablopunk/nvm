package services

import (
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
)

// TestOCRSourcesNameTheImageTheyMean checks the source forms 1.x accepts: a path, a file: URL (escaped), a path under "~",
// and an ExtensionFile's path or URL when the source is empty.
func TestOCRSourcesNameTheImageTheyMean(t *testing.T) {
	cases := []struct {
		name  string
		input api.OCRInput
		want  string
	}{
		{"absolute path", api.OCRInput{Source: testpath.Abs("tmp", "receipt.png")}, testpath.Abs("tmp", "receipt.png")},
		{"file URL", api.OCRInput{Source: fileURLFor(testpath.Abs("tmp", "my receipt.png"))}, testpath.Abs("tmp", "my receipt.png")},
		{"home path", api.OCRInput{Source: "~/receipt.png"}, testpath.Abs("home", "ada", "receipt.png")},
		{"file path", api.OCRInput{File: &api.FileRef{Path: testpath.Abs("tmp", "a.png")}}, testpath.Abs("tmp", "a.png")},
		{"file URL of a file", api.OCRInput{File: &api.FileRef{FileURL: fileURLFor(testpath.Abs("tmp", "b.png"))}}, testpath.Abs("tmp", "b.png")},
	}
	for _, c := range cases {
		if got := ocrPathOf(ocrSourceOf(c.input), testpath.Abs("home", "ada")); got != c.want {
			t.Errorf("%s: path = %q, want %q", c.name, got, c.want)
		}
	}
	if got := ocrPathOf("~/receipt.png", ""); got != "~/receipt.png" {
		t.Errorf("without a home folder the path = %q, want it unchanged", got)
	}
}

// TestOCRDataURLsOfAnyImageTypeAreRead checks a data URL of any image type is read, and a data URL of another media type
// or with invalid base64 is refused.
func TestOCRDataURLsOfAnyImageTypeAreRead(t *testing.T) {
	jpeg := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte("jpeg"))
	data, isData, err := ocrDataURLInput(jpeg)
	if err != nil || !isData || string(data) != "jpeg" {
		t.Fatalf("jpeg data URL = %q, %v, %v; want its bytes", data, isData, err)
	}
	if _, isData, _ := ocrDataURLInput(testpath.Abs("tmp", "a.png")); isData {
		t.Fatal("a path was read as a data URL")
	}
	if _, _, err := ocrDataURLInput("data:text/plain;base64,AA=="); err == nil {
		t.Fatal("a text data URL was accepted")
	}
	if _, _, err := ocrDataURLInput("data:image/png;base64,!!"); err == nil {
		t.Fatal("invalid base64 was accepted")
	}
}

// TestOCRReadsAFileURLAndAHomePath serves both forms through the service and gets the recognized text of the file.
func TestOCRReadsAFileURLAndAHomePath(t *testing.T) {
	host, deps := newDesktopHost(t, platform.CapabilityTextRecognize)
	home := t.TempDir()
	deps.Home = home
	image := filepath.Join(home, "receipt.png")
	host.text.Script(image, platform.TextResult{Text: "total 4"})
	h := desktopHandler(t, deps, api.CapabilityOCR)
	fileURL := fileURLFor(image)
	for _, source := range []string{fileURL, "~/receipt.png"} {
		var result api.OCRResult
		if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceOCR, protocol.OpImage, protocol.OCRImageArgs{Input: api.OCRInput{Source: source}})), &result); err != nil || result.Text != "total 4" {
			t.Fatalf("ocr of %q = %+v (%v); want the recognized text", source, result, err)
		}
	}
}
