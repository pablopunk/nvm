package chat

import (
	"errors"
	"testing"
)

func TestBuilderReservationExcludesOtherChatsUntilSettlement(t *testing.T) {
	f := newFixture(t)
	first, err := f.svc.NewChat(KindBuilder, TierSmart)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.svc.NewChat(KindBuilder, TierSmart)
	if err != nil {
		t.Fatal(err)
	}
	turn := NewBuilderTurn()
	r := &run{kind: KindBuilder, builderTurn: turn}
	f.svc.runs[first.ID] = r
	if err := f.svc.ReserveExtension(first.ID, "user.target", turn); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ReserveExtension(first.ID, "user.target", turn); err != nil {
		t.Fatalf("repeat reservation: %v", err)
	}
	if err := f.svc.Bind(second.ID, "user.target"); !errors.Is(err, ErrExtensionHasChat) {
		t.Fatalf("competing binding: %v", err)
	}
	otherTurn := NewBuilderTurn()
	f.svc.runs[second.ID] = &run{kind: KindBuilder, builderTurn: otherTurn}
	if err := f.svc.ReserveExtension(second.ID, "user.target", otherTurn); !errors.Is(err, ErrExtensionHasChat) {
		t.Fatalf("competing reservation: %v", err)
	}
	delete(f.svc.runs, second.ID)
	f.svc.settle(first.ID, r, Outcome{Ending: EndStopped})
	if err := f.svc.Bind(second.ID, "user.target"); err != nil {
		t.Fatalf("reservation retained after settlement: %v", err)
	}
}

func TestReservationRequiresActiveBuilderAndItsBoundTarget(t *testing.T) {
	f := newFixture(t)
	c, err := f.svc.NewChat(KindBuilder, TierSmart)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ReserveExtension(c.ID, "user.target", NewBuilderTurn()); err == nil {
		t.Fatal("idle chat reserved a target")
	}
	if err := f.svc.Bind(c.ID, "user.owned"); err != nil {
		t.Fatal(err)
	}
	turn := NewBuilderTurn()
	f.svc.runs[c.ID] = &run{kind: KindBuilder, builderTurn: turn}
	if err := f.svc.ReserveExtension(c.ID, "user.other", turn); !errors.Is(err, ErrExtensionHasChat) {
		t.Fatalf("bound target changed: %v", err)
	}
	if err := f.svc.ReserveExtension(c.ID, "user.owned", turn); err != nil {
		t.Fatal(err)
	}
	f.svc.doomed[c.ID] = true
	if err := f.svc.ReserveExtension(c.ID, "user.owned", turn); err == nil {
		t.Fatal("removed chat reserved a target")
	}
}

func TestDelayedReservationCannotClaimForReplacementTurn(t *testing.T) {
	f := newFixture(t)
	c, err := f.svc.NewChat(KindBuilder, TierSmart)
	if err != nil {
		t.Fatal(err)
	}
	oldTurn := NewBuilderTurn()
	f.svc.runs[c.ID] = &run{kind: KindBuilder, builderTurn: oldTurn}
	newTurn := NewBuilderTurn()
	f.svc.opts.BuilderTools = func(id string) Toolset {
		if id != c.ID {
			t.Fatalf("toolset chat = %q", id)
		}
		return Toolset{BuilderTurn: newTurn}
	}
	_, identity := f.svc.setupFor(c)
	if identity != newTurn {
		t.Fatal("setup discarded builder identity")
	}
	f.svc.runs[c.ID] = &run{kind: KindBuilder, builderTurn: identity}
	for _, stale := range []*BuilderTurn{nil, oldTurn, NewBuilderTurn()} {
		if err := f.svc.ReserveExtension(c.ID, "user.stale", stale); err == nil {
			t.Fatal("stale or foreign callback reserved replacement turn")
		}
	}
	if len(f.svc.runs[c.ID].extensionClaims) != 0 {
		t.Fatal("rejected callback changed replacement claims")
	}
	if err := f.svc.ReserveExtension(c.ID, "user.current", newTurn); err != nil {
		t.Fatalf("current turn refused: %v", err)
	}
}
