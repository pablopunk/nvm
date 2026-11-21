package onnxocr

import (
	"context"
	"fmt"
	"image"
	"math"
	"slices"

	ort "github.com/benedoc-inc/onnxer/onnxruntime"
)

// The detector's settings, as RapidOCR runs these models. maxDetectionSide keeps a large display at a cost that suits
// a keypress while leaving small text legible.
const (
	minDetectionSide = 736
	maxDetectionSide = 2560
	detectionStep    = 32

	pixelThreshold  = 0.3
	regionThreshold = 0.5
	unclipRatio     = 1.6
	minRegionSide   = 3

	// Regions on one line join when they overlap this much vertically and the gap is under this share of their height.
	sameLineOverlap = 0.6
	sameLineGap     = 0.8
)

// detectionSize is the size the detector sees: small images enlarged, huge ones reduced, in steps the network accepts.
func detectionSize(width, height int) (int, int) {
	scale := 1.0
	if shortest := min(width, height); shortest < minDetectionSide {
		scale = float64(minDetectionSide) / float64(shortest)
	}
	if longest := float64(max(width, height)) * scale; longest > maxDetectionSide {
		scale *= maxDetectionSide / longest
	}
	return roundToStep(float64(width) * scale), roundToStep(float64(height) * scale)
}

func roundToStep(value float64) int {
	return max(detectionStep, int(math.Round(value/detectionStep))*detectionStep)
}

// detect finds the text lines of picture, in reading order and in its pixels.
func detect(ctx context.Context, runtime *ort.Runtime, session *ort.Session, picture *image.RGBA) ([]image.Rectangle, error) {
	bounds := picture.Bounds()
	width, height := detectionSize(bounds.Dx(), bounds.Dy())
	data := make([]float32, 3*width*height)
	fillChannels(data, resized(picture, width, height), width)
	probability, shape, err := infer(ctx, runtime, session, data, []int64{1, 3, int64(height), int64(width)})
	if err != nil {
		return nil, fmt.Errorf("detect text: %w", err)
	}
	if len(shape) != 4 || int(shape[2]) != height || int(shape[3]) != width {
		return nil, fmt.Errorf("the text detector answered with shape %v", shape)
	}
	scaleX, scaleY := float64(bounds.Dx())/float64(width), float64(bounds.Dy())/float64(height)
	var regions []image.Rectangle
	for _, region := range findRegions(probability, width, height) {
		regions = append(regions, image.Rect(
			int(float64(region.Min.X)*scaleX), int(float64(region.Min.Y)*scaleY),
			int(math.Ceil(float64(region.Max.X)*scaleX)), int(math.Ceil(float64(region.Max.Y)*scaleY)),
		).Intersect(bounds))
	}
	return readingOrder(joinLines(regions)), nil
}

// findRegions turns the detector's probability map into text rectangles, each grown back to the size of its text
// because the network marks only the core of a line.
func findRegions(probability []float32, width, height int) []image.Rectangle {
	mask := dilated(probability, width, height)
	var regions []image.Rectangle
	stack := make([]int, 0, 1024)
	for start := range mask {
		if !mask[start] {
			continue
		}
		mask[start] = false
		stack = append(stack[:0], start)
		minX, minY, maxX, maxY := width, height, 0, 0
		var sum float32
		var count int
		for len(stack) > 0 {
			at := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := at%width, at/width
			minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x+1), max(maxY, y+1)
			sum += probability[at]
			count++
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= 0 && ny >= 0 && nx < width && ny < height && mask[ny*width+nx] {
						mask[ny*width+nx] = false
						stack = append(stack, ny*width+nx)
					}
				}
			}
		}
		if sum/float32(count) < regionThreshold || min(maxX-minX, maxY-minY) < minRegionSide {
			continue
		}
		regions = append(regions, unclipped(image.Rect(minX, minY, maxX, maxY), width, height))
	}
	return regions
}

// dilated is the thresholded map grown by one pixel, which closes the small gaps inside a line.
func dilated(probability []float32, width, height int) []bool {
	mask := make([]bool, width*height)
	for at, value := range probability {
		if value <= pixelThreshold {
			continue
		}
		x, y := at%width, at/width
		for ny := max(0, y-1); ny <= min(height-1, y+1); ny++ {
			for nx := max(0, x-1); nx <= min(width-1, x+1); nx++ {
				mask[ny*width+nx] = true
			}
		}
	}
	return mask
}

func unclipped(region image.Rectangle, width, height int) image.Rectangle {
	w, h := float64(region.Dx()), float64(region.Dy())
	grow := int(math.Round(w * h * unclipRatio / (2 * (w + h))))
	return region.Inset(-grow).Intersect(image.Rect(0, 0, width, height))
}

// joinLines merges regions that are pieces of one line, so a recognizer sees the whole line and its spaces.
func joinLines(regions []image.Rectangle) []image.Rectangle {
	merged := slices.Clone(regions)
	for changed := true; changed; {
		changed = false
		for i := 0; i < len(merged) && !changed; i++ {
			for j := i + 1; j < len(merged); j++ {
				if sameLine(merged[i], merged[j]) {
					merged[i] = merged[i].Union(merged[j])
					merged = slices.Delete(merged, j, j+1)
					changed = true
					break
				}
			}
		}
	}
	return merged
}

func sameLine(a, b image.Rectangle) bool {
	shortest := min(a.Dy(), b.Dy())
	overlap := min(a.Max.Y, b.Max.Y) - max(a.Min.Y, b.Min.Y)
	gap := max(a.Min.X, b.Min.X) - min(a.Max.X, b.Max.X)
	return float64(overlap) >= sameLineOverlap*float64(shortest) && float64(gap) < sameLineGap*float64(shortest)
}

// readingOrder sorts top to bottom, then left to right among regions on one row.
func readingOrder(regions []image.Rectangle) []image.Rectangle {
	ordered := slices.Clone(regions)
	slices.SortFunc(ordered, func(a, b image.Rectangle) int { return (a.Min.Y + a.Max.Y) - (b.Min.Y + b.Max.Y) })
	for start := 0; start < len(ordered); {
		end := start + 1
		for end < len(ordered) && (ordered[end].Min.Y+ordered[end].Max.Y)-(ordered[start].Min.Y+ordered[start].Max.Y) < ordered[start].Dy() {
			end++
		}
		slices.SortFunc(ordered[start:end], func(a, b image.Rectangle) int { return a.Min.X - b.Min.X })
		start = end
	}
	return ordered
}
