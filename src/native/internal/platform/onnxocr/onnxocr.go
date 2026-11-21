// Package onnxocr reads text from images with PP-OCR models on ONNX Runtime, the same way on every OS. It runs in a
// short child process of the app, so the models and the runtime are out of memory as soon as a read ends.
//
// Layer: platform. The package imports the ONNX Runtime binding, image codecs and the standard library.
package onnxocr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	ort "github.com/benedoc-inc/onnxer/onnxruntime"
)

// ChildFlag is the first argument that makes the binary read one image and exit.
const ChildFlag = "--nvm-ocr-child"

// runtimeAPIVersion is the ONNX Runtime C API the binding speaks; scripts/natives/fetch-ocr.sh ships that runtime.
const runtimeAPIVersion = 23

// minTextConfidence drops what the recognizer read from icons and other marks that are not text.
const minTextConfidence = 0.5

// Assets are the files a read needs, which the app ships in its resources.
type Assets struct {
	Library    string
	Detector   string
	Recognizer string
	Dictionary string
}

// Args are the child's arguments for reading the image at path.
func (a Assets) Args(path string) []string {
	return []string{ChildFlag, a.Library, a.Detector, a.Recognizer, a.Dictionary, path}
}

type box struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type block struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Box        box     `json:"box"`
}

type output struct {
	Text   string  `json:"text"`
	Blocks []block `json:"blocks"`
}

// Main runs a read when args ask for one. handled is false for any other invocation of the binary.
func Main(args []string, stdout, stderr io.Writer) (code int, handled bool) {
	if len(args) < 2 || args[1] != ChildFlag {
		return 0, false
	}
	if len(args) != 7 {
		fmt.Fprintln(stderr, "text recognition was started with the wrong arguments")
		return 2, true
	}
	assets := Assets{Library: args[2], Detector: args[3], Recognizer: args[4], Dictionary: args[5]}
	result, err := read(context.Background(), assets, args[6])
	if err == nil {
		err = json.NewEncoder(stdout).Encode(result)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1, true
	}
	return 0, true
}

func read(ctx context.Context, assets Assets, path string) (output, error) {
	picture, err := decodeImage(path)
	if err != nil {
		return output{}, err
	}
	characters, err := readDictionary(assets.Dictionary)
	if err != nil {
		return output{}, err
	}
	runtime, err := ort.NewRuntime(assets.Library, runtimeAPIVersion)
	if err != nil {
		return output{}, fmt.Errorf("load the text recognition runtime: %w", err)
	}
	defer runtime.Close()
	env, err := runtime.NewEnv("nevermind-ocr", ort.LoggingLevelError)
	if err != nil {
		return output{}, err
	}
	defer env.Close()
	detector, err := runtime.NewSession(env, assets.Detector, &ort.SessionOptions{IntraOpNumThreads: 2, InterOpNumThreads: 1})
	if err != nil {
		return output{}, fmt.Errorf("load the text detector: %w", err)
	}
	defer detector.Close()
	recognizer, err := runtime.NewSession(env, assets.Recognizer, &ort.SessionOptions{IntraOpNumThreads: 2, InterOpNumThreads: 1})
	if err != nil {
		return output{}, fmt.Errorf("load the text recognizer: %w", err)
	}
	defer recognizer.Close()

	regions, err := detect(ctx, runtime, detector, picture)
	if err != nil {
		return output{}, err
	}
	result := output{Blocks: []block{}}
	var lines []string
	for _, region := range regions {
		text, confidence, err := recognize(ctx, runtime, recognizer, picture, region, characters)
		if err != nil {
			return output{}, err
		}
		text = strings.TrimSpace(text)
		if text == "" || confidence < minTextConfidence {
			continue
		}
		lines = append(lines, text)
		result.Blocks = append(result.Blocks, block{Text: text, Confidence: confidence, Box: box{
			X: float64(region.Min.X), Y: float64(region.Min.Y), Width: float64(region.Dx()), Height: float64(region.Dy()),
		}})
	}
	result.Text = strings.Join(lines, "\n")
	return result, nil
}

// infer runs a one-input model and returns a copy of its first output.
func infer(ctx context.Context, runtime *ort.Runtime, session *ort.Session, data []float32, shape []int64) ([]float32, []int64, error) {
	input, err := ort.NewTensorValue(runtime, data, shape)
	if err != nil {
		return nil, nil, err
	}
	defer input.Close()
	outputs, err := session.Run(ctx, map[string]*ort.Value{session.InputNames()[0]: input})
	if err != nil {
		return nil, nil, err
	}
	for _, value := range outputs {
		defer value.Close()
	}
	values, outShape, err := ort.GetTensorData[float32](outputs[session.OutputNames()[0]])
	if err != nil {
		return nil, nil, err
	}
	return slices.Clone(values), slices.Clone(outShape), nil
}

// readDictionary lists the recognizer's characters. The model's last class is a space the file does not name.
func readDictionary(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read the text dictionary: %w", err)
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return append(lines, " "), nil
}
