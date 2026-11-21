package services

import (
	"context"
	"encoding/json"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// ShellAction uses the same declaration, quota, validation and executor as the direct shell service.
func (h *Handler) ShellAction(ctx context.Context, command api.ShellCommand) (api.ShellResult, error) {
	if err := (api.Action{Kind: api.ActionShellExec, Shell: &command}).Validate(); err != nil {
		return api.ShellResult{}, invalid("%s", err)
	}
	options := api.ShellOptions{CWD: command.CWD, Env: command.Env, TimeoutMs: command.TimeoutMs, OutputLimit: command.OutputLimit}
	var args any = protocol.ShellExecArgs{Command: command.Command, Args: command.Args, Options: options}
	op := protocol.OpExec
	if command.Script != "" {
		op = protocol.OpScript
		args = protocol.ShellScriptArgs{Script: command.Script, Options: options}
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return api.ShellResult{}, err
	}
	result, err := h.runProcess(ctx, protocol.ServiceCall{Service: protocol.ServiceDesktopShell, Op: op, Args: raw})
	if err != nil {
		return api.ShellResult{}, err
	}
	return result.(api.ShellResult), nil
}
