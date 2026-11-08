package manager

import (
	"context"
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/bundle"
)

func TestSnapshotIdentityChangesOnDisableAndReinstall(t *testing.T) {
	h := newHarness(t)
	const id = "acme.menubar"
	contents := fixtureContents(t, id)
	if err := h.m.Install(id, contents, true); err != nil {
		t.Fatal(err)
	}
	first := h.m.SnapshotIdentity(id)
	firstRuntime, err := h.m.runtimeFor(id)
	if err != nil {
		t.Fatal(err)
	}
	if first == nil {
		t.Fatal("enabled extension has no identity")
	}
	if err := h.m.Disable(id); err != nil {
		t.Fatal(err)
	}
	if h.m.SnapshotIdentity(id) != nil {
		t.Fatal("disabled extension exposed identity")
	}
	if err := h.m.Enable(id); err != nil {
		t.Fatal(err)
	}
	second := h.m.SnapshotIdentity(id)
	secondRuntime, err := h.m.runtimeFor(id)
	if err != nil {
		t.Fatal(err)
	}
	if firstRuntime == secondRuntime {
		t.Fatal("enable reused disabled runtime")
	}
	if second == nil || second == first {
		t.Fatal("enable reused disabled installation identity")
	}
	if err := h.m.Uninstall(id); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Install(id, contents, true); err != nil {
		t.Fatal(err)
	}
	third := h.m.SnapshotIdentity(id)
	if third == nil || third == second || third == first {
		t.Fatal("reinstall reused prior identity")
	}
}

func TestSnapshotRevisionRequiresCurrentFilesAuthority(t *testing.T) {
	h := newHarness(t)
	contents := fixtureContents(t, "acme.good")
	contents[bundle.ManifestFile] = []byte(`{"apiVersion":"1.0","id":"acme.good","title":"Good","capabilities":["desktop.files"]}`)
	if err := h.m.Install("acme.good", contents, true); err != nil {
		t.Fatal(err)
	}
	identity := h.m.SnapshotIdentity("acme.good")
	revision, ok := h.m.SnapshotRevision("acme.good", identity)
	if !ok || revision == "" {
		t.Fatal("enabled installation has no revision")
	}
	if _, ok := h.m.SnapshotRevision("acme.good", []string{"not comparable"}); ok {
		t.Fatal("invalid identity was accepted")
	}
	if err := h.m.Disable("acme.good"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.m.SnapshotRevision("acme.good", identity); ok {
		t.Fatal("disabled installation retained authority")
	}
	if err := h.m.Enable("acme.good"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.m.SnapshotRevision("acme.good", identity); ok {
		t.Fatal("old identity survived re-enablement")
	}
}

func TestLoaderRejectsReplacedInstallation(t *testing.T) {
	h := newHarness(t)
	const id = "acme.menubar"
	contents := fixtureContents(t, id)
	if err := h.m.Install(id, contents, true); err != nil {
		t.Fatal(err)
	}
	first := h.m.SnapshotIdentity(id)
	if err := h.m.Disable(id); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Enable(id); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.m.LoadForIdentity(context.Background(), id, "unused", first); err == nil {
		t.Fatal("loader accepted an obsolete installation identity")
	}
}

func TestViewActionRejectsReplacedInstallation(t *testing.T) {
	h := newHarness(t)
	const id = "acme.fallback"
	contents := fixtureContents(t, id)
	if err := h.m.Install(id, contents, true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning(id)
	first := h.m.SnapshotIdentity(id)
	if err := h.m.Disable(id); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Enable(id); err != nil {
		t.Fatal(err)
	}
	h.waitRunning(id)
	_, _, err := h.m.Run(context.Background(), id, api.Invocation{Ref: api.ActionRef{ID: "bump"}, RequireIdentity: true, ExpectedIdentity: first})
	if !errors.Is(err, ErrChangedInstallation) {
		t.Fatalf("old view action error=%v", err)
	}
}

func TestCapturedRuntimeCannotReachReenabledChild(t *testing.T) {
	h := newHarness(t)
	const id = "acme.fallback"
	if err := h.m.Install(id, fixtureContents(t, id), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning(id)
	oldRuntime, oldIdentity, err := h.m.runtimeForInvocation(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.m.Disable(id); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Enable(id); err != nil {
		t.Fatal(err)
	}
	h.waitRunning(id)
	newRuntime, newIdentity, err := h.m.runtimeForInvocation(id)
	if err != nil {
		t.Fatal(err)
	}
	if oldRuntime == newRuntime || oldIdentity == newIdentity {
		t.Fatal("reenable reused runtime or identity")
	}
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	if _, err := oldRuntime.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "bump"}}); err == nil {
		t.Fatal("captured old runtime dispatched after reenable")
	}
}

func TestSupervisedFollowUpRetainsOriginAcrossReplacement(t *testing.T) {
	h := newHarness(t)
	const id = "acme.fallback"
	if err := h.m.Install(id, fixtureContents(t, id), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning(id)
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	result, failure, err := h.m.Run(ctx, id, api.Invocation{Ref: api.ActionRef{ID: "next"}})
	if err != nil || failure != nil || result.Next == nil || !result.OriginBound {
		t.Fatalf("initial result=%+v failure=%v err=%v", result, failure, err)
	}
	follow := api.Invocation{Ref: *result.Next.Ref, RequireIdentity: true, ExpectedIdentity: result.OriginIdentity}
	if _, failure, err := h.m.Run(ctx, id, follow); err != nil || failure != nil {
		t.Fatalf("follow-up failure=%v err=%v", failure, err)
	}
	if err := h.m.Disable(id); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Enable(id); err != nil {
		t.Fatal(err)
	}
	h.waitRunning(id)
	if _, _, err := h.m.Run(ctx, id, follow); !errors.Is(err, ErrChangedInstallation) {
		t.Fatalf("stale follow-up error=%v", err)
	}
}

func TestPasteAdmissionRejectsReplacedInstallation(t *testing.T) {
	h := newHarness(t)
	const id = "acme.fallback"
	contents := fixtureContents(t, id)
	if err := h.m.Install(id, contents, true); err != nil {
		t.Fatal(err)
	}
	first := h.m.SnapshotIdentity(id)
	if err := h.m.Disable(id); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Enable(id); err != nil {
		t.Fatal(err)
	}
	err := h.m.PerformPasteForIdentity(id, first, api.PasteText("Paste", "fixture"))
	if !errors.Is(err, ErrChangedInstallation) {
		t.Fatalf("stale paste admission: %v", err)
	}
}
