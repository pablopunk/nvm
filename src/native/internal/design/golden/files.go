package golden

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

// update rewrites goldens instead of comparing them: go test -run '^TestName$' -update .
var update = flag.Bool("update", false, "rewrite golden PNGs under testdata/golden instead of comparing")

// goldenRoot is where goldens and failure artifacts live, relative to the test's package directory.
// Tests of this package point it at a temporary directory so that they never touch the repository goldens.
var goldenRoot = filepath.Join("testdata", "golden")

const failuresDirectory = "_failures"

func themeName(dark bool) string {
	if dark {
		return "dark"
	}
	return "light"
}

// variantOf names the appearance and scale of a render, as in light.1x.
func variantOf(opts Options) string {
	return fmt.Sprintf("%s.%gx", themeName(opts.Dark), opts.Scale)
}

func goldenPath(opts Options, name string) string {
	return filepath.Join(goldenRoot, opts.Group, name+"."+variantOf(opts)+".png")
}

func failureBase(opts Options, name string) string {
	return opts.Group + "." + name + "." + variantOf(opts)
}

func failurePath(opts Options, name, kind string) string {
	return filepath.Join(goldenRoot, failuresDirectory, failureBase(opts, name)+"."+kind+".png")
}

func diffPathFor(opts Options, name string) string {
	return failurePath(opts, name, "diff")
}

func packageDirectoryName() string {
	wd, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return filepath.Base(wd)
}

// readPNG decodes a golden into an RGBA image, so that pixels compare in one layout.
func readPNG(path string) (*image.RGBA, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoded, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode golden %s: %w", path, err)
	}
	bounds := decoded.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, decoded, bounds.Min, draw.Src)
	return rgba, nil
}

func writePNG(path string, img *image.RGBA) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return file.Close()
}

// writeFailure stores the actual frame and, when there is one, the diff image under _failures/. The
// directory ignores its own contents, so failures never reach a commit by accident. It returns the path
// of the actual frame.
func writeFailure(opts Options, name string, actual, diff *image.RGBA) (string, error) {
	dir := filepath.Join(goldenRoot, failuresDirectory)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ignore := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(ignore); err != nil {
		if err := os.WriteFile(ignore, []byte("*\n!.gitignore\n"), 0o644); err != nil {
			return "", err
		}
	}
	actualPath := failurePath(opts, name, "actual")
	if err := writePNG(actualPath, actual); err != nil {
		return "", err
	}
	if diff != nil {
		if err := writePNG(diffPathFor(opts, name), diff); err != nil {
			return "", err
		}
	}
	return actualPath, nil
}
