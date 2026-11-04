package classifyimage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/extai/extaitest"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/child"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// hostContext is the built-in's context over the host's real services: the AI call service on the fixture backend and
// the file service on the fake file system of the rig. Its calls take the same path a built-in's calls take in the app.
func hostContext(t *testing.T) (api.Context, *extaitest.Rig) {
	t.Helper()
	rig := extaitest.New(t)
	manifest := Extension{}.Manifest()
	handler := services.New(services.Deps{Clock: rig.Set.Clock, Files: rig.Set.FS, DataRoot: t.TempDir(), AI: rig.Service}, manifest)
	t.Cleanup(func() { _ = handler.Close() })
	serve := func(call protocol.ServiceCall) protocol.ServiceReply {
		return handler.Serve(context.Background(), call)
	}
	return child.NewDirectContext(manifest.Identity, rig.Set.Clock, serve).For(context.Background(), api.LaunchContext{}), rig
}

// TestClassifyOverTheHostServicesListsTheFixtureCategory runs the form's submit through the host's services and the
// fixture backend: the picture reaches the decision route typed by its extension, and the first category answers.
func TestClassifyOverTheHostServicesListsTheFixtureCategory(t *testing.T) {
	ctx, rig := hostContext(t)
	rig.Set.FS.Seed("/pics/cat.jpg", make([]byte, 64))
	result, err := Extension{}.RunAction(ctx, api.Invocation{
		Ref: api.ActionRef{ID: classifyAction}, Form: api.FormValues{"image": api.TextValue("/pics/cat.jpg")},
	})
	if err != nil || result.View == nil || len(result.View.Items) != 1 || result.View.Items[0].Title != "photograph" {
		t.Fatalf("result = %+v, %v; want the category photograph", result, err)
	}
	decisions := decisionRequests(rig)
	if len(decisions) != 1 || !strings.Contains(decisions[0], `"mediaType":"image/jpeg"`) {
		t.Fatalf("decision requests = %d, want one carrying the jpeg picture", len(decisions))
	}
}

// TestClassifyOverTheHostServicesRefusesBeforeTheModel checks the refusals through the host: nothing reaches the
// backend for a picture the route cannot take.
func TestClassifyOverTheHostServicesRefusesBeforeTheModel(t *testing.T) {
	cases := []struct {
		name string
		path string
		size int
		want error
	}{
		{"another format", "/pics/anim.gif", 64, errNoImage},
		{"unreadable", "/pics/missing.png", 0, errNoImage},
		{"too large", "/pics/big.png", MaximumImageBytes + 1, errImageTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, rig := hostContext(t)
			if tc.size > 0 {
				rig.Set.FS.Seed(tc.path, make([]byte, tc.size))
			}
			_, err := Extension{}.RunAction(ctx, api.Invocation{
				Ref: api.ActionRef{ID: classifyAction}, Form: api.FormValues{"image": api.TextValue(tc.path)},
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if got := len(decisionRequests(rig)); got != 0 {
				t.Fatalf("decision requests = %d, want none", got)
			}
		})
	}
}

// TestClassifyOverTheHostServicesNeedsASignedInAccount checks the sign-in copy when the account is signed out.
func TestClassifyOverTheHostServicesNeedsASignedInAccount(t *testing.T) {
	ctx, rig := hostContext(t)
	rig.Set.FS.Seed("/pics/cat.jpg", make([]byte, 64))
	rig.SignOut()
	_, err := Extension{}.RunAction(ctx, api.Invocation{
		Ref: api.ActionRef{ID: classifyAction}, Form: api.FormValues{"image": api.TextValue("/pics/cat.jpg")},
	})
	if !errors.Is(err, errNotSignedIn) {
		t.Fatalf("err = %v, want the sign-in copy", err)
	}
	if got := len(decisionRequests(rig)); got != 0 {
		t.Fatalf("decision requests = %d, want none while signed out", got)
	}
}

// decisionRequests returns the bodies the backend received on the decision route.
func decisionRequests(rig *extaitest.Rig) []string {
	var bodies []string
	for _, request := range rig.Backend.Requests() {
		if request.Path == fixtures.DecisionsPath {
			bodies = append(bodies, request.Body)
		}
	}
	return bodies
}
