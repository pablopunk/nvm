package services

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// TestPlatformTableNamesTheWireOperations keeps api.PlatformOperations in step with the protocol's names.
func TestPlatformTableNamesTheWireOperations(t *testing.T) {
	wire := map[string][2]string{
		"Desktop().Shell().AppleScript":              {protocol.ServiceDesktopShell, protocol.OpAppleScript},
		"Desktop().WindowManagement().SwitchDesktop": {protocol.ServiceDesktopWindows, protocol.OpSwitchDesktop},
		"Desktop().Apps().Hide":                      {protocol.ServiceDesktopApps, protocol.OpHideApp},
		"Desktop().Keyboard().TypeText":              {protocol.ServiceDesktopKeyboard, protocol.OpTypeText},
	}
	for _, operation := range api.PlatformOperations {
		want, ok := wire[operation.Name()]
		if !ok || operation.Service != want[0] || operation.Op != want[1] {
			t.Errorf("%s is served as %s/%s; the table says %s/%s", operation.Name(), want[0], want[1], operation.Service, operation.Op)
		}
	}
	if len(wire) != len(api.PlatformOperations) {
		t.Errorf("the table has %d operations; this test knows %d", len(api.PlatformOperations), len(wire))
	}
}

// TestAnOperationOfAnotherOSFailsTypedAndQuiet: AppleScript off macOS is ErrUnsupported on the wire, nothing runs, and
// the user is told nothing.
func TestAnOperationOfAnotherOSFailsTypedAndQuiet(t *testing.T) {
	commands, h, shown := notifiedHost(t, "linux")
	reply := serve(t, h, protocol.ServiceDesktopShell, protocol.OpAppleScript, protocol.ShellScriptArgs{Script: "beep"})
	if reply.Failure == nil || reply.Failure.Code != api.CodeUnsupported {
		t.Fatalf("reply = %+v; want an unsupported failure", reply)
	}
	if !errors.Is(failureErrorOf(reply.Failure), api.ErrUnsupported) {
		t.Fatalf("failure %+v does not map back to ErrUnsupported", reply.Failure)
	}
	if len(commands.Calls()) != 0 || len(*shown) != 0 {
		t.Fatalf("ran %d processes and showed %q; want nothing", len(commands.Calls()), *shown)
	}
}

// failureErrorOf rebuilds the sentinel the child gives an extension for a failure code.
func failureErrorOf(failure *api.Failure) error {
	if failure.Code == api.CodeUnsupported {
		return api.ErrUnsupported
	}
	return errors.New(failure.Message)
}
