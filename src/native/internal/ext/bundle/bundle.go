// Package bundle reads extensions from disk. An extension is a folder named after its id, holding
// extension.json (the declarative manifest the host reviews before anything runs) and main.go (the source
// the child interprets). Loading never runs extension code.
//
// Layer: ext. The package imports api, platform and the standard library only.
package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// File names inside an extension folder.
const (
	ManifestFile = "extension.json"
	SourceFile   = "main.go"
)

// MaxSourceBytes bounds the source the host reads. A larger file is a load problem, not a partial read.
const MaxSourceBytes = 1 << 20

// Bundle is one extension folder as the host sees it before enabling it. Problems lists why it cannot run;
// a bundle with problems is listed in the manager but never started.
type Bundle struct {
	Dir      string
	Manifest api.Manifest
	Source   []byte
	Hash     string
	Problems []string
}

// Runnable reports whether the bundle has no load problems.
func (b Bundle) Runnable() bool {
	return len(b.Problems) == 0
}

// Load reads one extension folder. It reports problems in Bundle.Problems rather than failing, so one
// broken folder never hides the others.
func Load(files platform.FS, dir string) Bundle {
	bundle := Bundle{Dir: dir}
	manifestBytes, err := files.ReadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		bundle.Problems = append(bundle.Problems, fmt.Sprintf("%s is missing or unreadable", ManifestFile))
		return bundle
	}
	manifest, err := decodeManifest(manifestBytes)
	if err != nil {
		bundle.Problems = append(bundle.Problems, err.Error())
		return bundle
	}
	bundle.Manifest = manifest
	if err := manifest.Validate(); err != nil {
		bundle.Problems = append(bundle.Problems, err.Error())
	}
	source, err := readSource(files, filepath.Join(dir, SourceFile))
	if err != nil {
		bundle.Problems = append(bundle.Problems, err.Error())
	}
	bundle.Source = source
	bundle.Hash = Hash(manifestBytes, source)
	return bundle
}

// decodeManifest reads extension.json. Unknown fields are ignored, as the API's compatibility rules say.
func decodeManifest(data []byte) (api.Manifest, error) {
	var manifest api.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return api.Manifest{}, fmt.Errorf("%s is not valid JSON: %w", ManifestFile, err)
	}
	return manifest, nil
}

func readSource(files platform.FS, path string) ([]byte, error) {
	source, size, err := files.ReadFilePrefix(path, MaxSourceBytes+1)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s is missing", SourceFile)
		}
		return nil, fmt.Errorf("%s is unreadable: %w", SourceFile, err)
	}
	if size > MaxSourceBytes {
		return nil, fmt.Errorf("%s is larger than %d bytes", SourceFile, MaxSourceBytes)
	}
	return source, nil
}

// Hash identifies one version of an extension. A change in either file changes the hash, so a reload
// can tell a new version from the one already running.
func Hash(manifest []byte, source []byte) string {
	sum := sha256.New()
	sum.Write(manifest)
	sum.Write([]byte{0})
	sum.Write(source)
	return hex.EncodeToString(sum.Sum(nil))
}

// Scan loads every extension folder under root, sorted by folder name. Hidden folders are staging and
// trash folders from install and are skipped. A missing root is an empty list, not an error.
func Scan(files platform.FS, root string) ([]Bundle, error) {
	entries, err := files.ReadDir(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var bundles []Bundle
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		bundle := Load(files, filepath.Join(root, entry.Name()))
		if len(bundle.Problems) == 0 && bundle.Manifest.ID != entry.Name() {
			bundle.Problems = append(bundle.Problems, fmt.Sprintf("folder name must match the id %q", bundle.Manifest.ID))
		}
		bundles = append(bundles, bundle)
	}
	return bundles, nil
}
