package app

import "testing"

func TestMeaningDeciderIsNilWithoutAnAccount(t *testing.T) {
	p := &App{}
	if p.meaningDecider() != nil {
		t.Fatal("a decider exists without an account; the provider would call the backend while signed out")
	}
}

func TestMeaningFilesCommandIsOfferedOnlyWhenSignedIn(t *testing.T) {
	p := &App{}
	if got := p.meaningFilesCommands(); len(got) != 0 {
		t.Fatalf("signed-out commands = %v, want none", got)
	}
}
