package builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
)

// Candidate is an extension the model proposed: the manifest it wrote, as text and decoded, and its source.
type Candidate struct {
	Manifest     api.Manifest
	ManifestText string
	Source       string
}

// Files returns the two files of the extension folder.
func (c Candidate) Files() map[string][]byte {
	return map[string][]byte{
		bundle.ManifestFile: []byte(c.ManifestText),
		bundle.SourceFile:   []byte(c.Source),
	}
}

// ErrNoBlocks is the failure of a reply without both code blocks.
var ErrNoBlocks = errors.New("the reply needs one ```json manifest block and one ```go source block")

// ParseReply reads the last manifest and the last source block of a reply and checks the manifest. The source is
// checked later, by the extension host, so this reads the reply only.
func ParseReply(text string) (Candidate, error) {
	blocks := codeBlocks(text)
	manifestText := lastBlockIn(blocks, "json")
	source := lastBlockIn(blocks, "go", "golang")
	if manifestText == "" || source == "" {
		return Candidate{}, ErrNoBlocks
	}
	var manifest api.Manifest
	if err := json.Unmarshal([]byte(manifestText), &manifest); err != nil {
		return Candidate{}, fmt.Errorf("the manifest is not valid JSON: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Candidate{}, fmt.Errorf("the manifest is invalid: %w", err)
	}
	return Candidate{Manifest: manifest, ManifestText: manifestText, Source: source}, nil
}

// codeBlock is one fenced block of a reply: its language tag and its body.
type codeBlock struct {
	lang string
	body string
}

// codeBlocks splits a reply into its fenced blocks. A fence closes on a line of its own, so two blocks in a row never
// read as one. A block the reply never closes is dropped, as a cut-off reply has no complete block.
func codeBlocks(text string) []codeBlock {
	var blocks []codeBlock
	var current *codeBlock
	var body strings.Builder
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case current == nil && strings.HasPrefix(trimmed, "```"):
			current = &codeBlock{lang: strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "```")))}
			body.Reset()
		case current != nil && trimmed == "```":
			current.body = body.String()
			blocks = append(blocks, *current)
			current = nil
		case current != nil:
			body.WriteString(line)
			body.WriteByte('\n')
		}
	}
	return blocks
}

// lastBlockIn returns the body of the last block whose language is one of langs, or "" when there is none.
func lastBlockIn(blocks []codeBlock, langs ...string) string {
	for i := len(blocks) - 1; i >= 0; i-- {
		if slices.Contains(langs, blocks[i].lang) {
			return blocks[i].body
		}
	}
	return ""
}
