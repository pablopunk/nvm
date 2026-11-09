//go:build darwin

package real

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

const (
	thumbnailSubcommand = "thumbnail"
	filePictureTimeout  = 25 * time.Second
)

// quickLookPictures draws files with Quick Look through the compiled helper.
type quickLookPictures struct {
	helper platform.Helper
	binary string
}

func newFilePictures(helper platform.Helper) platform.FilePictures {
	return quickLookPictures{helper: helper, binary: macosHelperPath()}
}

// FilePicture asks the helper for the picture and reads the file it wrote.
func (q quickLookPictures) FilePicture(ctx context.Context, path string, edge int) ([]byte, error) {
	dir, err := os.MkdirTemp("", "nvm-picture-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	output := filepath.Join(dir, "picture.png")
	ctx, cancel := context.WithTimeout(ctx, filePictureTimeout)
	defer cancel()
	result, err := q.helper.Run(ctx, q.binary, []string{thumbnailSubcommand, path, strconv.Itoa(edge), output}, nil)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, errors.New("quick look: " + strings.TrimSpace(string(result.Stderr)))
	}
	return os.ReadFile(output)
}
