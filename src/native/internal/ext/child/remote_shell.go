package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteShell opens web links and runs programs and scripts on the host. Each run is one host call under the
// host's deadline and output limit.
type remoteShell struct {
	link *hostLink
}

func (s remoteShell) OpenExternal(url string) error {
	return s.link.Call(protocol.ServiceDesktopShell, protocol.OpOpenExternal, protocol.URLArgs{URL: url}, nil)
}

func (s remoteShell) Exec(command string, args []string, options api.ShellOptions) (api.ShellResult, error) {
	var result api.ShellResult
	err := s.link.Call(protocol.ServiceDesktopShell, protocol.OpExec, protocol.ShellExecArgs{Command: command, Args: args, Options: options}, &result)
	return result, err
}

func (s remoteShell) Script(script string, options api.ShellOptions) (api.ShellResult, error) {
	var result api.ShellResult
	err := s.link.Call(protocol.ServiceDesktopShell, protocol.OpScript, protocol.ShellScriptArgs{Script: script, Options: options}, &result)
	return result, err
}

func (s remoteShell) AppleScript(script string, options api.ShellOptions) (api.ShellResult, error) {
	var result api.ShellResult
	err := s.link.Call(protocol.ServiceDesktopShell, protocol.OpAppleScript, protocol.ShellScriptArgs{Script: script, Options: options}, &result)
	return result, err
}

func (s remoteShell) Which(command string) (api.ShellResult, error) {
	var result api.ShellResult
	err := s.link.Call(protocol.ServiceDesktopShell, protocol.OpWhich, protocol.ShellWhichArgs{Command: command}, &result)
	return result, err
}
