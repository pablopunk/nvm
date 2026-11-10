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
	r := &run{kind: KindBuilder}
	f.svc.runs[first.ID] = r
	if err := f.svc.ReserveExtension(first.ID, "user.target"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ReserveExtension(first.ID, "user.target"); err != nil {
		t.Fatalf("repeat reservation: %v", err)
	}
	if err := f.svc.Bind(second.ID, "user.target"); !errors.Is(err, ErrExtensionHasChat) {
		t.Fatalf("competing binding: %v", err)
	}
	f.svc.runs[second.ID] = &run{kind: KindBuilder}
	if err := f.svc.ReserveExtension(second.ID, "user.target"); !errors.Is(err, ErrExtensionHasChat) {
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
	if err := f.svc.ReserveExtension(c.ID, "user.target"); err == nil {
		t.Fatal("idle chat reserved a target")
	}
	if err := f.svc.Bind(c.ID, "user.owned"); err != nil {
		t.Fatal(err)
	}
	f.svc.runs[c.ID] = &run{kind: KindBuilder}
	if err := f.svc.ReserveExtension(c.ID, "user.other"); !errors.Is(err, ErrExtensionHasChat) {
		t.Fatalf("bound target changed: %v", err)
	}
	if err := f.svc.ReserveExtension(c.ID, "user.owned"); err != nil {
		t.Fatal(err)
	}
	f.svc.doomed[c.ID] = true
	if err := f.svc.ReserveExtension(c.ID, "user.owned"); err == nil {
		t.Fatal("removed chat reserved a target")
	}
}
