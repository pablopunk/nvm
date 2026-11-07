package real

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// userPathTimeout bounds the login shell that reports the user's PATH.
const userPathTimeout = 5 * time.Second

// userPathDelimiter brackets the PATH in the login shell's output, so profile noise cannot be mistaken for it.
const userPathDelimiter = "__NVM_PATH_DELIM__"

// commands runs extension processes with the user's login PATH in front of the host's PATH, as the Electron app does.
type commands struct {
	userPath *userPathCache
}

// Run executes the program in spec, killing it when ctx ends. Output beyond MaxHelperOutputBytes is discarded.
func (c commands) Run(ctx context.Context, spec platform.CommandSpec) (platform.HelperResult, error) {
	env := mergedEnvironment(os.Environ(), c.userPath.get(ctx), spec.Env)
	binary, err := commandPath(spec.Binary, env)
	if err != nil {
		return platform.HelperResult{}, err
	}
	cmd := exec.CommandContext(ctx, binary, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = env
	cmd.WaitDelay = helperWaitDelay
	stdout := &limitedBuffer{limit: platform.MaxHelperOutputBytes}
	stderr := &limitedBuffer{limit: platform.MaxHelperOutputBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	result := platform.HelperResult{
		Stdout:    stdout.buf.Bytes(),
		Stderr:    stderr.buf.Bytes(),
		Truncated: stdout.truncated || stderr.truncated,
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, ctxErr
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}

func commandPath(binary string, env []string) (string, error) {
	if strings.ContainsAny(binary, `/\`) {
		return binary, nil
	}
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !(key == "PATH" || (runtime.GOOS == "windows" && strings.EqualFold(key, "PATH"))) {
			continue
		}
		for _, dir := range filepath.SplitList(value) {
			if !filepath.IsAbs(dir) {
				continue
			}
			if path, err := exec.LookPath(filepath.Join(dir, binary)); err == nil {
				return path, nil
			}
		}
	}
	return "", &exec.Error{Name: binary, Err: exec.ErrNotFound}
}

// mergedEnvironment returns base with the user PATH in front of PATH, then the extra entries. Later entries win.
func mergedEnvironment(base []string, userPath string, extra map[string]string) []string {
	values := map[string]string{}
	order := []string{}
	for _, entry := range base {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if _, seen := values[key]; !seen {
			order = append(order, key)
		}
		values[key] = value
	}
	if path := joinPathList(userPath, values["PATH"]); path != "" {
		if _, seen := values["PATH"]; !seen {
			order = append(order, "PATH")
		}
		values["PATH"] = path
	}
	for key, value := range extra {
		if _, seen := values[key]; !seen {
			order = append(order, key)
		}
		values[key] = value
	}
	env := make([]string, 0, len(order))
	for _, key := range order {
		env = append(env, key+"="+values[key])
	}
	return env
}

func joinPathList(first, second string) string {
	switch {
	case first == "":
		return second
	case second == "":
		return first
	}
	return first + string(os.PathListSeparator) + second
}

// userPathCache reads the login PATH once, from the user's shell, on first use.
type userPathCache struct {
	once  sync.Once
	value string
}

func newUserPathCache() *userPathCache {
	return &userPathCache{}
}

func (c *userPathCache) get(ctx context.Context) string {
	c.once.Do(func() { c.value = readUserPath(ctx) })
	return c.value
}

// readUserPath asks the user's login shell for its PATH. Windows and a missing SHELL have none to report.
func readUserPath(ctx context.Context) string {
	shell := os.Getenv("SHELL")
	if runtime.GOOS == "windows" || shell == "" {
		return ""
	}
	pathCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), userPathTimeout)
	defer cancel()
	script := "printf '%s%s%s' '" + userPathDelimiter + "' \"$PATH\" '" + userPathDelimiter + "'"
	out, err := exec.CommandContext(pathCtx, shell, "-ilc", script).Output()
	if err != nil {
		return ""
	}
	parts := strings.Split(string(bytes.TrimSpace(out)), userPathDelimiter)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
