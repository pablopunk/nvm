package real

import (
	"context"
	"errors"
	"os"
	"runtime"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/onnxocr"
)

// onnxText reads text in a child process of this binary, so the models leave memory when each read ends.
type onnxText struct {
	helper platform.Helper
}

func newTextRecognizer(helper platform.Helper) platform.TextRecognizer {
	return onnxText{helper: helper}
}

// Recognize reads the image at path. languages is not used: the model reads every Latin script at once.
func (t onnxText) Recognize(ctx context.Context, path string, _ []string) (platform.TextResult, error) {
	assets, ok := bundledOCRAssets()
	if !ok {
		return platform.TextResult{}, platform.ErrUnsupported
	}
	return t.read(ctx, assets, path)
}

func (t onnxText) read(ctx context.Context, assets onnxocr.Assets, path string) (platform.TextResult, error) {
	executable, err := os.Executable()
	if err != nil {
		return platform.TextResult{}, err
	}
	result, err := t.helper.Run(ctx, executable, assets.Args(path), nil)
	if err != nil {
		return platform.TextResult{}, err
	}
	if result.Truncated {
		return platform.TextResult{}, errUnreadableTextOutput
	}
	if result.ExitCode != 0 {
		return platform.TextResult{}, errors.New(recognitionFailure(result.Stderr))
	}
	return parseOCROutput(result.Stdout)
}

// bundledOCRAssets finds the runtime and models the app ships, and reports whether they are all there.
func bundledOCRAssets() (onnxocr.Assets, bool) {
	assets := onnxocr.Assets{
		Library:    existingBundledResource("lib", onnxLibraryName(runtime.GOOS)),
		Detector:   existingBundledResource("ocr", "det.onnx"),
		Recognizer: existingBundledResource("ocr", "rec.onnx"),
		Dictionary: existingBundledResource("ocr", "dict.txt"),
	}
	return assets, assets.Library != "" && assets.Detector != "" && assets.Recognizer != "" && assets.Dictionary != ""
}

func onnxLibraryName(goos string) string {
	switch goos {
	case "darwin":
		return "libonnxruntime.dylib"
	case "windows":
		return "onnxruntime.dll"
	}
	return "libonnxruntime.so"
}

func ocrAssetsAvailable() bool {
	_, ok := bundledOCRAssets()
	return ok
}
