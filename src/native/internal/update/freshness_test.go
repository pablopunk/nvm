package update_test

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/update"
	updatefake "github.com/pablopunk/nvm/native/internal/update/fake"
)

func TestCheckedAtRecordsWhenTheLastCheckFinished(t *testing.T) {
	clock := fake.NewClock()
	svc := update.NewService(update.Options{Engine: updatefake.NewEngine(nil), Clock: clock, Current: running})
	if !svc.CheckedAt().IsZero() {
		t.Fatalf("checked at = %v before any check, want zero", svc.CheckedAt())
	}
	svc.Check(context.Background())
	if got := svc.CheckedAt(); !got.Equal(clock.Now()) {
		t.Fatalf("checked at = %v, want %v", got, clock.Now())
	}
}

func TestCheckIfStaleChecksOnlyAnOldAnswer(t *testing.T) {
	clock := fake.NewClock()
	engine := updatefake.NewEngine(nil)
	svc := update.NewService(update.Options{Engine: engine, Clock: clock, Current: running})
	const maxAge = 10 * time.Minute

	if !svc.CheckIfStale(context.Background(), maxAge) || engine.Checks() != 1 {
		t.Fatalf("first open checked = %d times, want one check when nothing was checked yet", engine.Checks())
	}
	clock.Advance(maxAge)
	if svc.CheckIfStale(context.Background(), maxAge) || engine.Checks() != 1 {
		t.Fatalf("an answer %v old was checked again", maxAge)
	}
	clock.Advance(time.Second)
	if !svc.CheckIfStale(context.Background(), maxAge) || engine.Checks() != 2 {
		t.Fatalf("an answer older than %v was not checked again", maxAge)
	}
}

func TestSwitchingChannelForgetsTheLastCheck(t *testing.T) {
	svc := update.NewService(update.Options{Engine: updatefake.NewEngine(nil), Clock: fake.NewClock(), Channels: &memoryChannels{channel: update.ChannelStable}, Current: running})
	svc.Check(context.Background())
	if err := svc.SetChannel(update.ChannelDev); err != nil {
		t.Fatal(err)
	}
	if !svc.CheckedAt().IsZero() {
		t.Fatalf("checked at = %v after a channel switch, want zero", svc.CheckedAt())
	}
}

func TestCheckIfStaleDoesNothingInABuildThatCannotUpdate(t *testing.T) {
	engine := updatefake.Disabled()
	svc := update.NewService(update.Options{Engine: engine, Clock: fake.NewClock(), Current: running})
	if svc.CheckIfStale(context.Background(), time.Minute) || engine.Checks() != 0 {
		t.Fatal("a build that cannot update ran a check")
	}
}
