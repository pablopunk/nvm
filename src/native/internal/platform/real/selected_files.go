package real

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

const selectedFilesTimeout = 5 * time.Second

// The process identity is checked on both sides of the Finder read, without activating an app.
// JXA's JSON serializer frames paths containing newlines, quotes, and other special characters.
const selectedFilesScript = `function run(argv) {
  ObjC.import('Foundation');
  const pid = Number(argv[0]);
  const se = Application('System Events');
  function identity() {
    const processes = se.applicationProcesses.whose({unixId: pid})();
    if (processes.length !== 1) return null;
    return processes[0].bundleIdentifier();
  }
  const before = identity();
  if (!before) return JSON.stringify({status: 'missing'});
  if (before !== 'com.apple.finder') return JSON.stringify({status: 'other'});
  const finder = Application('Finder');
  const paths = finder.selection().map(item => ObjC.unwrap($.NSURL.URLWithString(item.url()).path));
  if (identity() !== before) return JSON.stringify({status: 'missing'});
  return JSON.stringify({status: 'ok', paths: paths});
}`

type selectedFilesReader struct{ helper platform.Helper }

type selectedFilesAnswer struct {
	Status string   `json:"status"`
	Paths  []string `json:"paths"`
}

func (r selectedFilesReader) Paths(ctx context.Context, target platform.FocusTarget) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.helper == nil {
		return nil, platform.ErrUnsupported
	}
	if target.PID <= 0 {
		return nil, platform.ErrNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, selectedFilesTimeout)
	defer cancel()
	result, err := r.helper.Run(ctx, osascriptBinary, []string{"-l", "JavaScript", "-e", selectedFilesScript, strconv.Itoa(target.PID)}, nil)
	if errors.Is(err, exec.ErrNotFound) {
		return nil, platform.ErrUnsupported
	}
	if err != nil {
		return nil, fmt.Errorf("selected files helper: %w", err)
	}
	if result.Truncated {
		return nil, errors.New("selected files helper output truncated")
	}
	if result.ExitCode != 0 {
		message := string(result.Stderr)
		if strings.Contains(message, "(-1743)") || strings.Contains(message, "(-1744)") || strings.Contains(message, "(-10004)") {
			return nil, fmt.Errorf("selected files automation: %w", platform.ErrDenied)
		}
		return nil, fmt.Errorf("selected files helper failed: %s", strings.TrimSpace(message))
	}
	var answer selectedFilesAnswer
	decoder := json.NewDecoder(strings.NewReader(string(result.Stdout)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&answer); err != nil {
		return nil, fmt.Errorf("selected files response: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("selected files response has trailing data")
	}
	switch answer.Status {
	case "missing":
		return nil, platform.ErrNotFound
	case "other":
		return []string{}, nil
	case "ok":
		if answer.Paths == nil {
			return nil, errors.New("selected files response missing paths")
		}
		for i, selectedPath := range answer.Paths {
			// Finder may include a trailing slash for directories.
			if selectedPath != "/" {
				selectedPath = strings.TrimSuffix(selectedPath, "/")
			}
			// This helper always returns macOS POSIX paths, including when its
			// contract is tested with a fake helper on another platform.
			if !path.IsAbs(selectedPath) || path.Clean(selectedPath) != selectedPath || strings.ContainsRune(selectedPath, 0) {
				return nil, errors.New("selected files response contains invalid path")
			}
			answer.Paths[i] = selectedPath
		}
		return answer.Paths, nil
	default:
		return nil, errors.New("selected files response has invalid status")
	}
}
