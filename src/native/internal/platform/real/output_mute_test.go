package real

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestMutedReadsTheVolumeSettingsAnswer(t *testing.T) {
	for answer, want := range map[string]bool{"true\n": true, "false\n": false} {
		helper := fake.NewHelper()
		helper.Script(osascriptBinary, fake.HelperScript{Stdout: []byte(answer)})
		got, err := osascriptOutputMute{helper: helper}.Muted()
		if err != nil || got != want {
			t.Fatalf("Muted() for %q = %v, %v; want %v", answer, got, err, want)
		}
		if args := helper.Calls()[0].Args; !strings.Contains(args[1], "output muted") {
			t.Fatalf("script = %v, want the output mute query", args)
		}
	}
}

func TestSetMutedRunsTheStatementForTheRequestedState(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(osascriptBinary, fake.HelperScript{}, fake.HelperScript{})
	mute := osascriptOutputMute{helper: helper}
	if err := mute.SetMuted(true); err != nil {
		t.Fatalf("SetMuted(true): %v", err)
	}
	if err := mute.SetMuted(false); err != nil {
		t.Fatalf("SetMuted(false): %v", err)
	}
	calls := helper.Calls()
	if calls[0].Args[1] != "set volume with output muted" || calls[1].Args[1] != "set volume without output muted" {
		t.Fatalf("calls = %+v, want the mute and unmute statements", calls)
	}
}

func TestUnreadableOrFailedOutputCallsAreErrors(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(osascriptBinary,
		fake.HelperScript{Stdout: []byte("maybe")},
		fake.HelperScript{ExitCode: 1, Stderr: []byte("not allowed")},
		fake.HelperScript{Err: exec.ErrNotFound},
	)
	mute := osascriptOutputMute{helper: helper}
	if _, err := mute.Muted(); err == nil {
		t.Fatalf("an answer that is not true or false was accepted")
	}
	if err := mute.SetMuted(true); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("SetMuted with a failed script = %v, want the script's error", err)
	}
	if err := mute.SetMuted(true); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("SetMuted without osascript = %v, want ErrUnsupported", err)
	}
}
