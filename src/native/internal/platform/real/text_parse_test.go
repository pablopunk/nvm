package real

import (
	"reflect"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestVisionOutputKeepsTextBlocksBoxesAndMeanConfidence(t *testing.T) {
	stdout := []byte(`{"text":"Total\n42","confidence":0.75,"language":"en-US","blocks":[` +
		`{"text":"Total","confidence":0.5,"box":{"x":10.4,"y":20,"width":60,"height":12.6}},` +
		`{"text":"42","confidence":1,"box":{"x":10,"y":40,"width":20,"height":12}}]}`)
	result, err := parseVisionOutput(stdout)
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

func TestVisionOutputWithoutLinesHasNoConfidence(t *testing.T) {
	result, err := parseVisionOutput([]byte(`{"text":"","confidence":null,"language":null,"blocks":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Confidence != 0 || len(result.Blocks) != 0 || result.Language != "" {
		t.Fatalf("empty result = %+v", result)
	}
}

func TestVisionOutputThatIsNotJSONIsUnreadable(t *testing.T) {
	if _, err := parseVisionOutput([]byte("Segmentation fault")); err != errUnreadableTextOutput {
		t.Fatalf("err = %v, want the unreadable-output error", err)
	}
}

func TestTesseractTSVGroupsWordsIntoLinesWithConfidencePercentages(t *testing.T) {
	tsv := strings.Join([]string{
		"level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext",
		"1\t1\t0\t0\t0\t0\t0\t0\t300\t200\t-1\t",
		"4\t1\t1\t1\t1\t0\t10\t20\t100\t15\t-1\t",
		"5\t1\t1\t1\t1\t1\t10\t20\t40\t15\t90\tInvoice",
		"5\t1\t1\t1\t1\t2\t55\t21\t30\t14\t80\t#42\twith tab",
		"5\t1\t1\t1\t2\t1\t10\t50\t50\t15\t-1\t ",
		"5\t1\t1\t1\t3\t1\t10\t70\t60\t16\t70\tTotal",
		"",
	}, "\n")
	result, err := parseTesseractTSV([]byte(tsv), []string{"eng"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Invoice #42\twith tab\nTotal" || result.Language != "eng" {
		t.Fatalf("text = %q language %q", result.Text, result.Language)
	}
	first := result.Blocks[0]
	if first.Box != (platform.Rect{X: 10, Y: 20, Width: 75, Height: 15}) || first.Confidence != 0.85 {
		t.Fatalf("first line = %+v", first)
	}
	if result.Blocks[1].Confidence != 0.7 {
		t.Fatalf("second line confidence = %v", result.Blocks[1].Confidence)
	}
	if got := result.Confidence; got < 0.77 || got > 0.78 {
		t.Fatalf("mean confidence = %v", got)
	}
}

func TestTesseractTSVWithoutWordsIsEmpty(t *testing.T) {
	result, err := parseTesseractTSV([]byte("level\tpage_num\n1\t1\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "" || len(result.Blocks) != 0 || result.Confidence != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestLanguageCodesThatCouldBeOptionsAreRefused(t *testing.T) {
	if _, err := checkLanguages([]string{"en-US", "fr"}); err != nil {
		t.Fatalf("valid codes refused: %v", err)
	}
	for _, bad := range []string{"-config", "eng; rm", "", "../x"} {
		if _, err := checkLanguages([]string{bad}); err == nil {
			t.Errorf("language %q accepted", bad)
		}
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
