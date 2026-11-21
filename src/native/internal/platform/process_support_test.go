package platform_test

import (
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"testing"
)

func TestProcessAvailabilityMatchesImplementedForms(t *testing.T) {
	commands := fake.NewCommands()
	for _, target := range []string{"darwin", "linux", "windows", "unknown"} {
		support := platform.ProcessAvailability(commands, target)
		if !support.Exec || support.LoginScript != (target == "darwin" || target == "linux") {
			t.Fatalf("%s: %+v", target, support)
		}
		if absent := platform.ProcessAvailability(nil, target); absent.Exec || absent.LoginScript {
			t.Fatalf("nil runner: %+v", absent)
		}
	}
	if len(commands.Calls()) != 0 {
		t.Fatal("support probe ran a process")
	}
}
