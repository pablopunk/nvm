package builder

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
)

// maxExtraSources bounds the Go files an extension may add beside main.go.
const maxExtraSources = 16

// extraSourceName is a plain Go file name, such as helpers.go. Test files and folders are not extension sources.
var extraSourceName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*\.go$`)

// Candidate is an extension the model proposed through write_extension: the manifest it wrote, as text and decoded,
// and its sources. Source is main.go; Sources holds any other Go files of the folder, by name.
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

// NewCandidate checks the arguments of write_extension and returns the candidate they describe. The manifest must be a
// valid manifest object; each extra file must have a plain .go name. The source is checked later, by the extension
// host, so this reads the arguments only.
func NewCandidate(manifestText, source string, extras map[string]string) (Candidate, error) {
	var manifest api.Manifest
	if err := json.Unmarshal([]byte(manifestText), &manifest); err != nil {
		return Candidate{}, fmt.Errorf("the manifest is not valid JSON: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Candidate{}, fmt.Errorf("the manifest is invalid: %w", err)
	}
	if strings.TrimSpace(source) == "" {
		return Candidate{}, fmt.Errorf("the source is empty; write main.go with package main and an entry point")
	}
	sources, err := checkExtraSources(extras)
	if err != nil {
		return Candidate{}, err
	}
	return Candidate{Manifest: manifest, ManifestText: manifestText, Source: source, Sources: sources}, nil
}

// checkExtraSources refuses a name that is not a plain .go file, and more files than maxExtraSources.
func checkExtraSources(extras map[string]string) (map[string]string, error) {
	for name := range extras {
		if name == bundle.SourceFile || !extraSourceName.MatchString(name) || strings.HasSuffix(name, "_test.go") {
			return nil, fmt.Errorf("the source file name %q must be a plain .go name such as helpers.go", name)
		}
	}
	if len(extras) > maxExtraSources {
		return nil, fmt.Errorf("an extension may add at most %d Go files beside main.go", maxExtraSources)
	}
	return maps.Clone(extras), nil
}
