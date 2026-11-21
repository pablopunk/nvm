package supervisor

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/extai/extaitest"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// aiServicesRig is the services rig of one extension whose model calls go to the AI rig's fixture backend.
func aiServicesRig(t *testing.T, aiRig *extaitest.Rig) servicesRig {
	t.Helper()
	rig := newServicesRig(t)
	rig.deps.AI = aiRig.Service
	return rig
}

// TestAIFixtureCallsReachTheBackendThroughTheChild runs the ai fixture in a child process: its ask and its stream
// reach the fixture backend through the protocol, and a burst past the 30 calls a minute is refused.
func TestAIFixtureCallsReachTheBackendThroughTheChild(t *testing.T) {
	aiRig := extaitest.New(t)
	aiRig.Backend.SetReply("pong from the fixture")
	runtime := servicedRuntime(t, "acme.ai", aiServicesRig(t, aiRig), fixtureManifest("acme.ai", api.CapabilityAI))

	if got := rowTitle(t, runtime, "ask"); got != "ask: pong from the fixture" {
		t.Fatalf("ask row %q", got)
	}
	stream := rowTitle(t, runtime, "stream")
	if !strings.HasPrefix(stream, "stream: pong from the fixture events=start,") || !strings.HasSuffix(stream, ",done") {
		t.Fatalf("stream row %q; want the reply with start and done events", stream)
	}
	// The ask and the stream above used two of the 30 calls of the minute, so the refusal lands on call 29.
	if got := rowTitle(t, runtime, "ratelimit"); !strings.Contains(got, "call 29 failed") || !strings.Contains(got, "AI rate limit exceeded") {
		t.Fatalf("ratelimit row %q; want the 30th call of the minute refused", got)
	}
}

// TestAIRefusedWithoutTheCapabilityInTheChild shows the refusal names the ai capability when the manifest omits it.
func TestAIRefusedWithoutTheCapabilityInTheChild(t *testing.T) {
	aiRig := extaitest.New(t)
	runtime := servicedRuntime(t, "acme.ai", aiServicesRig(t, aiRig), fixtureManifest("acme.ai"))
	if got := rowTitle(t, runtime, "ask"); !strings.Contains(got, "did not declare the ai capability") {
		t.Fatalf("row %q; want the undeclared-capability refusal", got)
	}
	if len(aiRig.Backend.Requests()) != 0 {
		t.Fatal("a refused call reached the backend")
	}
}

// TestSignedOutAIFixtureAsksForSignIn shows a signed-out account is told to sign in, not given a backend error.
func TestSignedOutAIFixtureAsksForSignIn(t *testing.T) {
	aiRig := extaitest.New(t)
	aiRig.SignOut()
	runtime := servicedRuntime(t, "acme.ai", aiServicesRig(t, aiRig), fixtureManifest("acme.ai", api.CapabilityAI))
	if got := rowTitle(t, runtime, "ask"); !strings.Contains(got, "Sign in") {
		t.Fatalf("row %q; want the sign-in line", got)
	}
}
