package builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
)

// maxExtraSources bounds the Go files a reply may add beside main.go.
const maxExtraSources = 16

// extraSourceName is a plain Go file name, such as helpers.go. Test files and folders are not extension sources.
var extraSourceName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*\.go$`)

// Candidate is an extension the model proposed: the manifest it wrote, as text and decoded, and its sources. Source
// is main.go; Sources holds any other Go files of the folder, by name.
type Candidate struct {
	Manifest     api.Manifest
	ManifestText string
	Source       string
	Sources      map[string]string
}

// Files returns every file of the extension folder: the manifest, main.go and each other Go file.
func (c Candidate) Files() map[string][]byte {
	files := map[string][]byte{
		bundle.ManifestFile: []byte(c.ManifestText),
		bundle.SourceFile:   []byte(c.Source),
	}
	for name, text := range c.Sources {
		files[name] = []byte(text)
	}
	return files
}

// SourceFiles returns the candidate's Go files by name, main.go included, as the folder would hold them.
func (c Candidate) SourceFiles() map[string][]byte {
	files := map[string][]byte{bundle.SourceFile: []byte(c.Source)}
	for name, text := range c.Sources {
		files[name] = []byte(text)
	}
	return files
}

// ErrNoBlocks is the failure of a reply without both code blocks.
var ErrNoBlocks = errors.New("the reply needs one ```json manifest block and one ```go source block")

// ParseReply reads the last manifest and the last main source block of a reply, checks the manifest, and keeps each
// named Go block (```go helpers.go) as an extra file. The source is checked later, by the extension host, so this
// reads the reply only.
func ParseReply(text string) (Candidate, error) {
	blocks := codeBlocks(text)
	manifestText := lastBlockIn(blocks, "json")
	source := lastMainSource(blocks)
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
	extras, err := namedSources(blocks)
	if err != nil {
		return Candidate{}, err
	}
	return Candidate{Manifest: manifest, ManifestText: manifestText, Source: source, Sources: extras}, nil
}

// codeBlock is one fenced block of a reply: its language tag, the file name written after the tag (if any) and its
// body.
type codeBlock struct {
	lang string
	name string
	body string
}

// lastMainSource is the body of the last Go block that is main.go: one that names no file or names main.go.
func lastMainSource(blocks []codeBlock) string {
	for i := len(blocks) - 1; i >= 0; i-- {
		if isGoBlock(blocks[i]) && (blocks[i].name == "" || blocks[i].name == bundle.SourceFile) {
			return blocks[i].body
		}
	}
	return ""
}

func isGoBlock(block codeBlock) bool {
	return block.lang == "go" || block.lang == "golang"
}

// namedSources maps each Go block that names a file to its body. A name that is not a plain .go file name is refused.
func namedSources(blocks []codeBlock) (map[string]string, error) {
	sources := map[string]string{}
	for _, block := range blocks {
		if !isGoBlock(block) || block.name == "" || block.name == bundle.SourceFile {
			continue
		}
		if !extraSourceName.MatchString(block.name) || strings.HasSuffix(block.name, "_test.go") {
			return nil, fmt.Errorf("the source file name %q must be a plain .go name such as helpers.go", block.name)
		}
		sources[block.name] = block.body
	}
	if len(sources) > maxExtraSources {
		return nil, fmt.Errorf("a reply may add at most %d Go files beside main.go", maxExtraSources)
	}
	return maps.Clone(sources), nil
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
			current = newCodeBlock(strings.TrimSpace(strings.TrimPrefix(trimmed, "```")))
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

// newCodeBlock reads a fence's info string: the language, then an optional file name. Only the language is lower-cased.
func newCodeBlock(info string) *codeBlock {
	words := strings.Fields(info)
	block := &codeBlock{}
	if len(words) > 0 {
		block.lang = strings.ToLower(words[0])
	}
	if len(words) > 1 {
		block.name = words[1]
	}
	return block
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
