package real

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMergedEnvironmentPutsTheUserPathInFrontAndExtrasLast(t *testing.T) {
	base := []string{"HOME=/Users/acme", "PATH=/usr/bin:/bin"}
	env := mergedEnvironment(base, "/opt/homebrew/bin", map[string]string{"MODE": "fast", "HOME": "/tmp/home"})
	values := map[string]string{}
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	wantPath := "/opt/homebrew/bin" + string(os.PathListSeparator) + "/usr/bin:/bin"
	if values["PATH"] != wantPath || values["MODE"] != "fast" || values["HOME"] != "/tmp/home" {
		t.Fatalf("environment = %v; want PATH %q, MODE and HOME overridden", values, wantPath)
	}
}

func TestCommandPathUsesTheChildEnvironmentInsteadOfTheDesktopPath(t *testing.T) {
	dir := t.TempDir()
	name := "nvm-fixture-gh"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	got, err := commandPath(name, []string{"PATH=" + dir})
	if err != nil || got != path {
		t.Fatalf("command lookup = %q, %v; want the login-environment executable %q", got, err, path)
	}
}

func TestMergedEnvironmentWithoutAUserPathKeepsTheHostPath(t *testing.T) {
	env := mergedEnvironment([]string{"PATH=/usr/bin"}, "", nil)
	if len(env) != 1 || env[0] != "PATH=/usr/bin" {
		t.Fatalf("environment = %v; want the host PATH unchanged", env)
	}
}
