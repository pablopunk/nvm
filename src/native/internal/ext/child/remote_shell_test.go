package child

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// shellLink answers one shell call with the reply and records the call the child sent.
func shellLink(t *testing.T, reply protocol.ServiceReply) (*hostLink, *protocol.ServiceCall) {
	t.Helper()
	var sent protocol.ServiceCall
	link := newDirectLink(func(call protocol.ServiceCall) protocol.ServiceReply {
		sent = call
		return reply
	})
	return link, &sent
}

func TestShellExecSendsTheProgramAndDecodesTheResult(t *testing.T) {
	link, sent := shellLink(t, protocol.ServiceReply{Data: json.RawMessage(`{"stdout":"hi","stderr":"","exitCode":0}`)})
	result, err := remoteShell{link: link}.Exec("gh", []string{"auth", "status"}, api.ShellOptions{TimeoutMs: 5000, Shell: true})
	if err != nil || result.Stdout != "hi" || result.ExitCode != 0 {
		t.Fatalf("Exec = %+v, %v; want the host's stdout", result, err)
	}
	if sent.Service != protocol.ServiceDesktopShell || sent.Op != protocol.OpExec {
		t.Fatalf("sent %s.%s; want desktop.shell exec", sent.Service, sent.Op)
	}
	var args protocol.ShellExecArgs
	if err := json.Unmarshal(sent.Args, &args); err != nil {
		t.Fatalf("decode args: %v", err)
	}
	if args.Command != "gh" || len(args.Args) != 2 || args.Options.TimeoutMs != 5000 || !args.Options.Shell {
		t.Fatalf("args = %+v; want the command, arguments and options", args)
	}
}

func TestShellOperationsUseTheirOwnOperationNames(t *testing.T) {
	cases := map[string]func(remoteShell) error{
		protocol.OpScript:      func(s remoteShell) error { _, err := s.Script("true", api.ShellOptions{}); return err },
		protocol.OpAppleScript: func(s remoteShell) error { _, err := s.AppleScript("return 1", api.ShellOptions{}); return err },
		protocol.OpWhich:       func(s remoteShell) error { _, err := s.Which("gh"); return err },
	}
	for op, run := range cases {
		link, sent := shellLink(t, protocol.ServiceReply{Data: json.RawMessage(`{"stdout":"","stderr":"","exitCode":0}`)})
		if err := run(remoteShell{link: link}); err != nil {
			t.Fatalf("%s: %v", op, err)
		}
		if sent.Op != op {
			t.Fatalf("sent op %q; want %q", sent.Op, op)
		}
	}
}

func TestShellRefusalIsAPermissionError(t *testing.T) {
	link, _ := shellLink(t, protocol.ServiceReply{Failure: &api.Failure{Code: api.CodePermission, Message: "no system capability"}})
	_, err := remoteShell{link: link}.Exec("gh", nil, api.ShellOptions{})
	if !errors.Is(err, api.ErrPermission) {
		t.Fatalf("error = %v; want the permission sentinel", err)
	}
}
