package real

import (
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// errUnreadableTextOutput is returned when an engine prints something other than the result it was asked for.
var errUnreadableTextOutput = errors.New("text recognition returned an unreadable result")

// recognitionFailure is the engine's own message when it gave one, and a plain statement otherwise.
func recognitionFailure(stderr []byte) string {
	if message := strings.TrimSpace(string(stderr)); message != "" {
		return message
	}
	return "text recognition failed"
}

// ocrOutput is what the OCR child prints: one JSON object with boxes in image pixels, origin top left.
type ocrOutput struct {
	Text       string   `json:"text"`
	Confidence *float64 `json:"confidence"`
	Language   *string  `json:"language"`
	Blocks     []struct {
		Text       string  `json:"text"`
		Confidence float64 `json:"confidence"`
		Box        struct {
			X      float64 `json:"x"`
			Y      float64 `json:"y"`
			Width  float64 `json:"width"`
			Height float64 `json:"height"`
		} `json:"box"`
	} `json:"blocks"`
}

// parseOCROutput converts the recognizer's JSON into a TextResult. A missing confidence means the engine had none.
func parseOCROutput(stdout []byte) (platform.TextResult, error) {
	var out ocrOutput
	if err := json.Unmarshal(stdout, &out); err != nil {
		return platform.TextResult{}, errUnreadableTextOutput
	}
	result := platform.TextResult{Text: out.Text}
	if out.Language != nil {
		result.Language = *out.Language
	}
	var sum float64
	for _, block := range out.Blocks {
		result.Blocks = append(result.Blocks, platform.TextBlock{
			Text:       block.Text,
			Confidence: block.Confidence,
			Box:        platform.Rect{X: round(block.Box.X), Y: round(block.Box.Y), Width: round(block.Box.Width), Height: round(block.Box.Height)},
		})
		sum += block.Confidence
	}
	if len(result.Blocks) > 0 {
		result.Confidence = sum / float64(len(result.Blocks))
	}
	return result, nil
}

func round(value float64) int { return int(math.Round(value)) }
