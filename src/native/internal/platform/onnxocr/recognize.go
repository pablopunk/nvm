package onnxocr

import (
	"context"
	"fmt"
	"image"
	"math"
	"strings"

	ort "github.com/benedoc-inc/onnxer/onnxruntime"
)

const (
	recognitionHeight   = 48
	minRecognitionWidth = 320
	maxRecognitionWidth = 3200
)

// recognize reads the one line of text inside region.
func recognize(ctx context.Context, runtime *ort.Runtime, session *ort.Session, picture *image.RGBA, region image.Rectangle, characters []string) (string, float64, error) {
	if region.Empty() {
		return "", 0, nil
	}
	width := min(maxRecognitionWidth, max(1, int(math.Ceil(float64(region.Dx())*recognitionHeight/float64(region.Dy())))))
	stride := max(width, minRecognitionWidth)
	data := make([]float32, 3*recognitionHeight*stride)
	fillChannels(data, resized(picture.SubImage(region), width, recognitionHeight), stride)
	scores, shape, err := infer(ctx, runtime, session, data, []int64{1, 3, recognitionHeight, int64(stride)})
	if err != nil {
		return "", 0, fmt.Errorf("recognize text: %w", err)
	}
	if len(shape) != 3 || int(shape[2]) != len(characters)+1 {
		return "", 0, fmt.Errorf("the text recognizer answered with shape %v for %d characters", shape, len(characters))
	}
	text, confidence := decodeCTC(scores, int(shape[1]), int(shape[2]), characters)
	return text, confidence, nil
}

// decodeCTC takes the likeliest class of each step, drops repeats and the blank class 0, and averages the kept scores.
func decodeCTC(scores []float32, steps, classes int, characters []string) (string, float64) {
	var text strings.Builder
	var total float64
	var kept, previous int
	for step := range steps {
		row := scores[step*classes : (step+1)*classes]
		best := 0
		for class, score := range row {
			if score > row[best] {
				best = class
			}
		}
		if best != 0 && best != previous {
			text.WriteString(characters[best-1])
			total += float64(row[best])
			kept++
		}
		previous = best
	}
	if kept == 0 {
		return "", 0
	}
	return text.String(), total / float64(kept)
}
