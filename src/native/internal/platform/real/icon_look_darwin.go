//go:build darwin

package real

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

const (
	iconSubcommand  = "icon"
	iconLookTimeout = 5 * time.Second
	// iconLookPrefix keeps the name of a look non-empty when the user set none of its preferences.
	iconLookPrefix = "icon look"
)

// iconLookPreferences start the global preferences that change how the system draws icons.
var iconLookPreferences = []string{"AppleIconAppearance", "AppleInterfaceStyle"}

// iconLookReader reads the look from the global preferences, again only after their file changes.
type iconLookReader struct {
	helper platform.Helper

	mu       sync.Mutex
	known    bool
	launch   string
	current  string
	modified time.Time
}

// newIconLookReader starts reading the look at once, so the look at launch is known before any change.
func newIconLookReader(helper platform.Helper) *iconLookReader {
	reader := &iconLookReader{helper: helper, current: iconLookPrefix, launch: iconLookPrefix}
	go reader.read(context.Background())
	return reader
}

func (r *iconLookReader) read(ctx context.Context) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	modified := globalPreferencesModified()
	if r.known && !modified.IsZero() && modified.Equal(r.modified) {
		return r.current, r.current != r.launch
	}
	ctx, cancel := context.WithTimeout(ctx, iconLookTimeout)
	defer cancel()
	result, err := r.helper.Run(ctx, "/usr/bin/defaults", []string{"read", "-g"}, nil)
	if err != nil || result.ExitCode != 0 {
		return r.current, r.current != r.launch
	}
	r.modified, r.current = modified, iconLookOf(string(result.Stdout))
	if !r.known {
		r.known, r.launch = true, r.current
	}
	return r.current, r.current != r.launch
}

func globalPreferencesModified() time.Time {
	home, err := os.UserHomeDir()
	if err != nil {
		return time.Time{}
	}
	info, err := os.Stat(filepath.Join(home, "Library/Preferences/.GlobalPreferences.plist"))
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// iconLookOf keeps the lines of the preferences that change how icons are drawn.
func iconLookOf(preferences string) string {
	look := []string{iconLookPrefix}
	for _, line := range strings.Split(preferences, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range iconLookPreferences {
			if strings.HasPrefix(line, prefix) {
				look = append(look, line)
			}
		}
	}
	return strings.Join(look, "\n")
}

// IconLook names the look now and reports whether it differs from the look at launch.
func (q quickLookPictures) IconLook(ctx context.Context) (string, bool) {
	return q.looks.read(ctx)
}

// ThemedIcon draws the icon in a new process, the only one that sees the look now.
func (q quickLookPictures) ThemedIcon(ctx context.Context, path string, size int) ([]byte, error) {
	return q.drawnByHelper(ctx, iconSubcommand, path, size)
}
