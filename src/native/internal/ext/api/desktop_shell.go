package api

// ShellOptions controls a process. Zero TimeoutMs and OutputLimit take the host defaults: 30 s, clamped to
// 1 s to 120 s, and 200000 characters per stream. Shell makes Exec join the command and its arguments into one
// line for /bin/sh -c; Script uses /bin/bash as a login shell on macOS and Linux.
// POSIX shell forms are unavailable on Windows; plain argv Exec is portable.
type ShellOptions struct {
	CWD         string            `json:"cwd,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	TimeoutMs   int64             `json:"timeoutMs,omitempty"`
	OutputLimit int64             `json:"outputLimit,omitempty"`
	Shell       bool              `json:"shell,omitempty"`
}

// ShellResult is the captured output of a process.
type ShellResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// Shell runs processes. Declare system; the host enforces that declaration.
// Probe shell.exec or shell.script for structural support, independently of permission or tool readiness.
type Shell interface {
	OpenExternal(url string) error
	// Exec resolves bare program names against the merged user-login PATH and Options.Env, including desktop launches.
	// Exec, Script, AppleScript and Which share a budget of budget.ShellRunsPerMinute runs per minute for the extension.
	// To check many items, run one Script that loops over them rather than one Exec per item: a run over the budget
	// fails with a quota error.
	Exec(command string, args []string, options ShellOptions) (ShellResult, error)
	Script(script string, options ShellOptions) (ShellResult, error)
	AppleScript(script string, options ShellOptions) (ShellResult, error)
	Which(command string) (ShellResult, error)
}
