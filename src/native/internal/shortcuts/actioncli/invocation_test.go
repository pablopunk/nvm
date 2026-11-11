package actioncli

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestInvocationQuotesForTheShell(t *testing.T) {
	cases := map[string]string{
		"command:quick-notes":            "nevermind-next run command:quick-notes",
		"app:/Applications/Some App.app": "nevermind-next run 'app:/Applications/Some App.app'",
		"x'; rm -rf ~; echo '":           `nevermind-next run 'x'\''; rm -rf ~; echo '\'''`,
	}
	for id, want := range cases {
		if got := Invocation("nevermind-next", id, "linux"); got != want {
			t.Errorf("%q: %s", id, got)
		}
	}
	if got := Invocation(`C:\Program Files\Nevermind\nvm.exe`, "a", "windows"); got != `"C:\Program Files\Nevermind\nvm.exe" run a` {
		t.Errorf("windows: %s", got)
	}
}

// Each quoted word reaches the program as exactly the original text.
func TestPosixQuoteRoundTripsThroughSh(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no POSIX shell")
	}
	for _, s := range []string{"plain", "with space", "it's", `"$HOME" $(id) ` + "`id`", "a\\b", "*", "", "ünïcode"} {
		out, err := exec.Command("sh", "-c", "printf %s "+PosixQuote(s)).Output()
		if err != nil || string(out) != s {
			t.Errorf("%q came back as %q (%v)", s, out, err)
		}
	}
}
