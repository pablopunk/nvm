package extview

import (
	"context"
	"fmt"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
	"maps"
	"slices"
	"strconv"
	"strings"
)

func frozenShellAction(action api.Action) api.Action {
	if action.Shell == nil {
		return action
	}
	action.Requires = slices.Clone(action.Requires)
	command := *action.Shell
	command.Args, command.Env = slices.Clone(command.Args), maps.Clone(command.Env)
	action.Shell = &command
	if action.Confirm != nil {
		confirmation := *action.Confirm
		action.Confirm = &confirmation
		return action
	}
	text := command.Script
	if text == "" {
		words := []string{strconv.Quote(command.Command)}
		for _, arg := range command.Args {
			words = append(words, strconv.Quote(arg))
		}
		text = strings.Join(words, " ")
	}
	if command.CWD != "" {
		text += "\nWorking folder: " + command.CWD
	}
	action.Confirm = &api.Confirmation{Title: "Run command?", ConfirmLabel: "Run", Message: text}
	return action
}

func (s *Session) runShellAction(h search.Host, r route) {
	if r.action.Shell == nil || s.deps.ShellAction == nil {
		h.Toast(unavailableMessage)
		return
	}
	current := func() bool { return !s.dismissed }
	if loading, ok := h.(search.ActionLoadingHost); ok {
		current = loading.BeginAction()
	}
	ctx := s.workContext
	detached := r.action.Background || r.action.DismissAfterRun
	if detached {
		ctx = context.Background()
	}
	if r.action.DismissAfterRun {
		h.Hide()
	}
	s.deps.ShellAction(ctx, r.extID, *r.action.Shell, r.identity, func(result api.ShellResult, err error) {
		visible := current()
		if s.dismissed || !visible {
			return
		}
		if err != nil {
			h.Toast(err.Error())
			return
		}
		if detached {
			h.Toast(fmt.Sprintf("Command exited with code %d", result.ExitCode))
			return
		}
		text := fmt.Sprintf("Exit code: %d\n\nSTDOUT\n%s\n\nSTDERR\n%s", result.ExitCode, result.Stdout, result.Stderr)
		view := api.View{Kind: api.ViewPreview, Title: orDefault(r.action.Title, "Command"), Preview: &api.PreviewSpec{Text: text}}
		child := s.navigationChild(r.extID, view, api.NavPush)
		h.Push(child.Open(h))
	})
}
