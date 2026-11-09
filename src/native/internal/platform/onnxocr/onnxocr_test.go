package onnxocr

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestDetectionSizeEnlargesSmallImagesAndCapsHugeOnes(t *testing.T) {
	if w, h := detectionSize(832, 582); h != 736 || w%detectionStep != 0 {
		t.Fatalf("small image = %dx%d", w, h)
	}
	if w, h := detectionSize(5120, 2880); w != maxDetectionSide || h%detectionStep != 0 {
		t.Fatalf("large display = %dx%d", w, h)
	}
}

func TestPiecesOfALineJoinAndColumnsStayApart(t *testing.T) {
	got := joinLines([]image.Rectangle{image.Rect(0, 0, 40, 20), image.Rect(45, 1, 90, 21), image.Rect(400, 0, 480, 20)})
	want := []image.Rectangle{image.Rect(0, 0, 90, 21), image.Rect(400, 0, 480, 20)}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestReadingOrderGoesDownThenAcross(t *testing.T) {
	got := readingOrder([]image.Rectangle{image.Rect(200, 2, 300, 22), image.Rect(0, 50, 100, 70), image.Rect(0, 0, 100, 20)})
	want := []image.Rectangle{image.Rect(0, 0, 100, 20), image.Rect(200, 2, 300, 22), image.Rect(0, 50, 100, 70)}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestDecodeDropsBlanksAndRepeats(t *testing.T) {
	scores := []float32{
		0.1, 0.9, 0, // a
		0.1, 0.9, 0, // a again, a repeat
		0.9, 0.1, 0, // blank
		0.1, 0.9, 0, // a, after a blank
		0, 0.1, 0.9, // b
	}
	if text, confidence := decodeCTC(scores, 5, 3, []string{"a", "b"}); text != "aab" || confidence < 0.89 {
		t.Fatalf("%q %v", text, confidence)
	}
}

func TestMainIgnoresOtherInvocations(t *testing.T) {
	if _, handled := Main([]string{"nvm"}, nil, nil); handled {
		t.Fatal("a normal start was taken for a read")
	}
}

// TestReadsSpanishWithItsAccents needs the runtime and models from scripts/natives/fetch-ocr.sh.
func TestReadsSpanishWithItsAccents(t *testing.T) {
	root := filepath.Join("..", "..", "..", "resources", runtime.GOOS+"-"+runtime.GOARCH)
	assets := Assets{
		Library:  filepath.Join(root, "lib", testLibraryName()),
		Detector: filepath.Join(root, "ocr", "det.onnx"), Recognizer: filepath.Join(root, "ocr", "rec.onnx"),
		Dictionary: filepath.Join(root, "ocr", "dict.txt"),
	}
	if _, err := os.Stat(assets.Library); err != nil {
		t.Skip("the OCR runtime is not fetched")
	}
	got, err := read(context.Background(), assets, filepath.Join("testdata", "spanish.png"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Configuración de la canción", "año pasado", "señal, niño, corazón", "Última actualización", "Búsqueda en español"} {
		if !strings.Contains(got.Text, want) {
			t.Errorf("missing %q in:\n%s", want, got.Text)
		}
	}
}

func testLibraryName() string {
	switch runtime.GOOS {
	case "darwin":
		return "libonnxruntime.dylib"
	case "windows":
		return "onnxruntime.dll"
	default:
		return "libonnxruntime.so"
	}
}
