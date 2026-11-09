package supervisor

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestDormantRuntimeAnswersFromItsDeclaredMethods(t *testing.T) {
	r := New(Config{Clock: fake.NewClock()}, api.Identity{ID: "acme.timer"}, "/ext/main.go", nil)
	if r.Provides(protocol.MethodRun) {
		t.Fatal("an idle runtime claimed a method")
	}
	r.StartDormant([]string{protocol.MethodRun})
	if got := r.Status().State; got != StateDormant {
		t.Fatalf("state %s", got)
	}
	if !r.Provides(protocol.MethodRun) || r.Provides(protocol.MethodSearch) {
		t.Fatal("a dormant runtime must answer from its declared methods")
	}
}
