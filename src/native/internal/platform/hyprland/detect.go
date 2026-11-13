package hyprland

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Runner runs a program and returns its standard output and exit code. A program that cannot start is an error.
type Runner func(ctx context.Context, name string, args ...string) (stdout []byte, exitCode int, err error)

// Env is what detection and the lifecycle read: the environment, the home folder, the compositor's command line
// (nil when it cannot be read), the program runner, and where Nevermind keeps the undo record.
type Env struct {
	Getenv         func(string) string
	Home           string
	CompositorArgs func() ([]string, error)
	Run            Runner
	UndoFile       string
}

// Session is what detection found. Problem is empty when the module can be set up automatically; otherwise it says
// why not, and Manual says whether the user can still set it up by hand.
type Session struct {
	Running      bool
	Version      Version
	Format       Format
	Entry        string
	ModulePath   string
	VerifyConfig bool
	Problem      string
	Manual       bool
}

// Ready reports whether setup, apply, undo and removal may write files.
func (s Session) Ready() bool { return s.Running && s.Problem == "" }

// Support is what can be written for this session.
func (s Session) Support() Support { return SupportFor(s.Version, s.Format) }

const probeTimeout = 3 * time.Second

// Detect finds the running Hyprland, its version and its config entry. It runs hyprctl and Hyprland --help, each with
// a short deadline; callers run it off the UI thread and cache the result.
func Detect(ctx context.Context, env Env) Session {
	if env.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return Session{Problem: "Hyprland is not running in this session"}
	}
	s := Session{Running: true}
	out, code, err := run(ctx, env, "hyprctl", "version", "-j")
	if err != nil || code != 0 {
		s.Problem = fmt.Sprintf("hyprctl version failed (%s)", describeFailure(code, err))
		return s
	}
	if s.Version, err = parseVersionJSON(out); err != nil {
		s.Problem = err.Error()
		return s
	}
	if !s.Version.AtLeast(MinimumVersion) {
		s.Problem = fmt.Sprintf("Hyprland %s is older than %s, the oldest version Nevermind writes config for", s.Version, MinimumVersion)
		return s
	}
	s.Entry, s.Format, s.Problem = findEntry(env, s.Version)
	if s.Problem != "" {
		s.Manual = s.Entry != ""
		return s
	}
	s.ModulePath = filepath.Join(filepath.Dir(s.Entry), moduleBase+s.Format.Extension())
	if err := checkWritable(s.Entry); err != nil {
		s.Problem, s.Manual = "the Hyprland config is read-only: "+err.Error(), true
		return s
	}
	if s.Format == FormatConf {
		s.VerifyConfig = verifyConfigAvailable(ctx, env)
	}
	return s
}

// verifyConfigAvailable reports whether Hyprland offers --verify-config. It is used for the classic format only:
// Hyprland 0.57 segfaults on exit when it checks a Lua config, even a valid one, and every crash shows the user a crash
// notification. A Lua change is checked by the reload and "hyprctl configerrors" instead, with a rollback.
func verifyConfigAvailable(ctx context.Context, env Env) bool {
	help, _, err := run(ctx, env, "Hyprland", "--help")
	return err == nil && strings.Contains(string(help), "--verify-config")
}

func run(ctx context.Context, env Env, name string, args ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	return env.Run(ctx, name, args...)
}

func describeFailure(code int, err error) string {
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("exit status %d", code)
}

// findEntry is the config file Hyprland loads: the one its --config argument names, else hyprland.lua or
// hyprland.conf in $XDG_CONFIG_HOME/hypr. It returns a problem when the file or its format cannot be told.
func findEntry(env Env, v Version) (string, Format, string) {
	if custom := customConfig(env); custom != "" {
		switch {
		case !v.AtLeast(luaConfig):
			return custom, FormatConf, ""
		case strings.HasSuffix(custom, ".lua"):
			return custom, FormatLua, ""
		case strings.HasSuffix(custom, ".conf"):
			return custom, FormatConf, ""
		}
		return custom, "", "Hyprland loads " + custom + ", whose format Nevermind cannot tell from its name"
	}
	base := env.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(env.Home, ".config")
	}
	lua, conf := filepath.Join(base, "hypr", "hyprland.lua"), filepath.Join(base, "hypr", "hyprland.conf")
	hasLua, hasConf := exists(lua), exists(conf)
	switch {
	case !v.AtLeast(luaConfig) && hasConf:
		return conf, FormatConf, ""
	case !v.AtLeast(luaConfig):
		return "", "", "no hyprland.conf was found in " + filepath.Dir(conf)
	case hasLua && hasConf:
		return lua, "", "both hyprland.lua and hyprland.conf exist, and Nevermind cannot tell which one Hyprland loads"
	case hasLua:
		return lua, FormatLua, ""
	case hasConf:
		return conf, FormatConf, ""
	}
	return "", "", "no hyprland.lua or hyprland.conf was found in " + filepath.Dir(lua)
}

// customConfig is the value of the compositor's -c or --config argument, or "" when it has none or it cannot be read.
func customConfig(env Env) string {
	if env.CompositorArgs == nil {
		return ""
	}
	args, err := env.CompositorArgs()
	if err != nil {
		return ""
	}
	for i, arg := range args {
		if value, ok := strings.CutPrefix(arg, "--config="); ok {
			return expandHome(value, env.Home)
		}
		if (arg == "-c" || arg == "--config") && i+1 < len(args) {
			return expandHome(args[i+1], env.Home)
		}
	}
	return ""
}

func expandHome(path, home string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return path
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// checkWritable checks that the entry file and its folder take writes, without changing either.
func checkWritable(entry string) error {
	target, err := filepath.EvalSymlinks(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	f.Close()
	probe, err := os.CreateTemp(filepath.Dir(entry), ".nevermind-probe-*")
	if err != nil {
		return err
	}
	name := probe.Name()
	probe.Close()
	return os.Remove(name)
}
