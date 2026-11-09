package real

import (
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestOCROutputKeepsTextBlocksBoxesAndMeanConfidence(t *testing.T) {
	stdout := []byte(`{"text":"Total\n42","confidence":0.75,"language":"en-US","blocks":[` +
		`{"text":"Total","confidence":0.5,"box":{"x":10.4,"y":20,"width":60,"height":12.6}},` +
		`{"text":"42","confidence":1,"box":{"x":10,"y":40,"width":20,"height":12}}]}`)
	result, err := parseOCROutput(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Total\n42" || result.Language != "en-US" || result.Confidence != 0.75 {
		t.Fatalf("result = %+v", result)
	}
	want := []platform.TextBlock{
		{Text: "Total", Confidence: 0.5, Box: platform.Rect{X: 10, Y: 20, Width: 60, Height: 13}},
		{Text: "42", Confidence: 1, Box: platform.Rect{X: 10, Y: 40, Width: 20, Height: 12}},
	}
	if !reflect.DeepEqual(result.Blocks, want) {
		t.Fatalf("blocks = %+v, want %+v", result.Blocks, want)
	}
}

func TestOCROutputWithoutLinesHasNoConfidence(t *testing.T) {
	result, err := parseOCROutput([]byte(`{"text":"","confidence":null,"language":null,"blocks":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Confidence != 0 || len(result.Blocks) != 0 || result.Language != "" {
		t.Fatalf("empty result = %+v", result)
	}
}

func TestOCROutputThatIsNotJSONIsUnreadable(t *testing.T) {
	if _, err := parseOCROutput([]byte("Segmentation fault")); err != errUnreadableTextOutput {
		t.Fatalf("err = %v, want the unreadable-output error", err)
	}
}

func TestRecognitionFailureUsesTheEnginesMessage(t *testing.T) {
	if got := recognitionFailure([]byte("  The image could not be read\n")); got != "The image could not be read" {
		t.Fatalf("message = %q", got)
	}
	if got := recognitionFailure(nil); got != "text recognition failed" {
		t.Fatalf("fallback = %q", got)
	}
}
