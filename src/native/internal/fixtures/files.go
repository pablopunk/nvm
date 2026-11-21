package fixtures

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// DefaultFileCount is the number of files Files returns when asked for zero or fewer.
const DefaultFileCount = 5000

// File-tree generator seeds. Changing them changes every generated path, so they stay fixed.
const (
	fileSeedHigh = 0x6e766d66
	fileSeedLow  = 0x66697465
)

// Name pools for the generated tree.
var (
	projectNames  = []string{"nvm", "atlas", "lumen", "ferry", "quarry", "sidecar", "tidepool", "marlin"}
	sourceFolders = []string{"src", "lib", "components", "internal", "tests", "docs", "scripts", "assets", "views"}
	stemNames     = []string{"index", "main", "config", "router", "store", "utils", "README", "notes", "draft",
		"invoice", "meeting", "budget", "report", "design", "retro", "plan", "todo", "helpers", "client", "server"}
	sourceExts   = []string{".ts", ".tsx", ".go", ".md", ".json", ".css", ".py", ".rs", ".swift", ".yaml", ".sh"}
	documentExts = []string{".pdf", ".docx", ".txt", ".md", ".xlsx", ".pages"}
	imageExts    = []string{".jpg", ".png", ".heic", ".gif"}
	appDirs      = []string{"Code", "Slack", "Raycast", "git", "gh", "fish", "zed"}
	packageNames = []string{"react", "lodash", "esbuild", "zod", "vitest", "yaml", "chalk", "minimist", "semver", "undici"}
)

// Files returns n file paths of a virtual home directory, sorted by generation order, not by name. The tree
// mixes project sources, documents, downloads, pictures, hidden configuration and cache folders, and ignored
// folders (node_modules and .git) at realistic depths. Paths are absolute and slash-separated. Zero or a
// negative n returns DefaultFileCount paths.
func Files(n int) []string {
	if n <= 0 {
		n = DefaultFileCount
	}
	rng := rand.New(rand.NewPCG(fileSeedHigh, fileSeedLow))
	seen := make(map[string]bool, n)
	paths := make([]string, 0, n)
	for len(paths) < n {
		path := generatePath(rng)
		if seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths
}

// SeedFiles creates every path in fsys: parent folders first, then a small file whose content is its path.
// It returns an error for the first path that cannot be created.
func SeedFiles(fsys *fake.FS, paths []string) error {
	for _, path := range paths {
		if err := fsys.MkdirAll(parentOf(path), 0o755); err != nil {
			return err
		}
		if err := fsys.WriteFile(path, []byte(path+"\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// generatePath draws one path from the weighted kinds of file a home directory holds.
func generatePath(rng *rand.Rand) string {
	switch roll := rng.IntN(100); {
	case roll < 55:
		return projectSourcePath(rng)
	case roll < 65:
		return "/home/user/Documents/" + pick(rng, stemNames) + pick(rng, documentExts)
	case roll < 75:
		return fmt.Sprintf("/home/user/Downloads/%s-%d%s", pick(rng, stemNames), 1+rng.IntN(999), pick(rng, documentExts))
	case roll < 80:
		return fmt.Sprintf("/home/user/Pictures/%d/IMG_%04d%s", 2020+rng.IntN(7), rng.IntN(10000), pick(rng, imageExts))
	case roll < 85:
		return fmt.Sprintf("/home/user/.config/%s/%s.json", pick(rng, appDirs), pick(rng, stemNames))
	case roll < 90:
		return nodeModulesPath(rng)
	case roll < 95:
		return gitObjectPath(rng)
	default:
		return fmt.Sprintf("/home/user/.cache/%s/%08x.bin", pick(rng, appDirs), rng.Uint32())
	}
}

// projectSourcePath is a source file one to four folders below a project root.
func projectSourcePath(rng *rand.Rand) string {
	var parts []string
	parts = append(parts, "/home/user/Projects", pick(rng, projectNames))
	depth := 1 + rng.IntN(4)
	for range depth {
		parts = append(parts, pick(rng, sourceFolders))
	}
	return strings.Join(parts, "/") + "/" + pick(rng, stemNames) + pick(rng, sourceExts)
}

// nodeModulesPath is a file inside an installed package, two to five folders deep.
func nodeModulesPath(rng *rand.Rand) string {
	parts := []string{"/home/user/Projects", pick(rng, projectNames), "node_modules", pick(rng, packageNames)}
	for range 2 + rng.IntN(4) {
		parts = append(parts, pick(rng, []string{"dist", "lib", "esm", "types", "utils"}))
	}
	return strings.Join(parts, "/") + "/" + pick(rng, stemNames) + ".js"
}

// gitObjectPath is a loose object in a repository's object store.
func gitObjectPath(rng *rand.Rand) string {
	return fmt.Sprintf("/home/user/Projects/%s/.git/objects/%02x/%038x", pick(rng, projectNames), rng.IntN(256), rng.Uint64())
}

// pick returns a uniformly chosen element of items.
func pick(rng *rand.Rand, items []string) string {
	return items[rng.IntN(len(items))]
}

// parentOf returns the folder that holds path.
func parentOf(path string) string {
	if i := strings.LastIndex(path, "/"); i > 0 {
		return path[:i]
	}
	return "/"
}
