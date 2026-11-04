package real

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// tesseractBinary is the Tesseract command line. Its presence on the PATH is the text.recognize capability on Linux.
const tesseractBinary = "tesseract"

// errUnreadableTextOutput is returned when an engine prints something other than the result it was asked for.
var errUnreadableTextOutput = errors.New("text recognition returned an unreadable result")

// languageCode accepts the BCP 47 and engine language names the palette and extensions use. Anything else, including a
// leading dash, is refused before it reaches an engine's command line.
var languageCode = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+-]{0,63}$`)

// recognitionFailure is the engine's own message when it gave one, and a plain statement otherwise.
func recognitionFailure(stderr []byte) string {
	if message := strings.TrimSpace(string(stderr)); message != "" {
		return message
	}
	return "text recognition failed"
}

// checkLanguages returns langs when every one is a plain language code, and an error naming the first that is not.
func checkLanguages(langs []string) ([]string, error) {
	for _, lang := range langs {
		if !languageCode.MatchString(lang) {
			return nil, errors.New("text recognition refused the language " + strconv.Quote(lang))
		}
	}
	return langs, nil
}

// visionOutput is what the macOS recognizer prints: one JSON object with boxes in image pixels, origin top left.
type visionOutput struct {
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

// parseVisionOutput converts the recognizer's JSON into a TextResult. A missing confidence means the engine had none.
func parseVisionOutput(stdout []byte) (platform.TextResult, error) {
	var out visionOutput
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

// tesseractColumns is the column count of a `tesseract ... tsv` row; the text is the last one and may hold tabs.
const tesseractColumns = 12

// tesseractWordLevel is the level of a word row in the TSV; other rows are pages, blocks, paragraphs and lines.
const tesseractWordLevel = "5"

// tesseractLine groups the words of one recognized line while the TSV is read.
type tesseractLine struct {
	words                  []string
	confidenceSum          float64
	left, top, right, base float64
}

// parseTesseractTSV converts `tesseract <image> stdout tsv` into a TextResult, one block per line of text. Word
// confidences are percentages in the TSV and are returned between 0 and 1.
func parseTesseractTSV(stdout []byte, languages []string) (platform.TextResult, error) {
	lines := map[string]*tesseractLine{}
	var order []string
	rows := strings.Split(string(stdout), "\n")
	for _, row := range rows[min(1, len(rows)):] {
		fields := strings.SplitN(strings.TrimRight(row, "\r"), "\t", tesseractColumns)
		if len(fields) != tesseractColumns || fields[0] != tesseractWordLevel || strings.TrimSpace(fields[11]) == "" {
			continue
		}
		key := fields[2] + "/" + fields[3] + "/" + fields[4]
		line, ok := lines[key]
		if !ok {
			line = &tesseractLine{left: math.Inf(1), top: math.Inf(1), right: math.Inf(-1), base: math.Inf(-1)}
			lines[key] = line
			order = append(order, key)
		}
		addTesseractWord(line, fields)
	}
	result := platform.TextResult{}
	if len(languages) > 0 {
		result.Language = languages[0]
	}
	var sum float64
	var texts []string
	for _, key := range order {
		line := lines[key]
		text := strings.Join(line.words, " ")
		confidence := line.confidenceSum / float64(len(line.words)) / 100
		texts = append(texts, text)
		sum += confidence
		result.Blocks = append(result.Blocks, platform.TextBlock{
			Text:       text,
			Confidence: confidence,
			Box:        platform.Rect{X: round(line.left), Y: round(line.top), Width: round(line.right - line.left), Height: round(line.base - line.top)},
		})
	}
	result.Text = strings.Join(texts, "\n")
	if len(result.Blocks) > 0 {
		result.Confidence = sum / float64(len(result.Blocks))
	}
	return result, nil
}

// addTesseractWord adds one word row to its line: its text, its confidence and its extent.
func addTesseractWord(line *tesseractLine, fields []string) {
	confidence, err := strconv.ParseFloat(fields[10], 64)
	if err != nil || confidence < 0 {
		confidence = 0
	}
	left, _ := strconv.ParseFloat(fields[6], 64)
	top, _ := strconv.ParseFloat(fields[7], 64)
	width, _ := strconv.ParseFloat(fields[8], 64)
	height, _ := strconv.ParseFloat(fields[9], 64)
	line.words = append(line.words, strings.TrimSpace(fields[11]))
	line.confidenceSum += confidence
	line.left = math.Min(line.left, left)
	line.top = math.Min(line.top, top)
	line.right = math.Max(line.right, left+width)
	line.base = math.Max(line.base, top+height)
}

// round rounds a pixel value to the nearest whole pixel.
func round(value float64) int {
	return int(math.Round(value))
}
