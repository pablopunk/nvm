package platform_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestAdoptRenamedSecretMovesTheEarlierEntryOnce(t *testing.T) {
	kc := fake.NewKeychain()
	_ = kc.Set("old", "acct", []byte("secret"))
	if err := platform.AdoptRenamedSecret(kc, "old", "new", "acct"); err != nil {
		t.Fatal(err)
	}
	if got, err := kc.Get("new", "acct"); err != nil || !bytes.Equal(got, []byte("secret")) {
		t.Fatalf("new = %q, %v; want the earlier secret", got, err)
	}
	if _, err := kc.Get("old", "acct"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("old entry kept: %v", err)
	}
}

func TestAdoptRenamedSecretKeepsTheCurrentEntry(t *testing.T) {
	kc := fake.NewKeychain()
	_ = kc.Set("old", "acct", []byte("earlier"))
	_ = kc.Set("new", "acct", []byte("current"))
	if err := platform.AdoptRenamedSecret(kc, "old", "new", "acct"); err != nil {
		t.Fatal(err)
	}
	if got, _ := kc.Get("new", "acct"); !bytes.Equal(got, []byte("current")) {
		t.Fatalf("new = %q; want the current secret untouched", got)
	}
	if got, _ := kc.Get("old", "acct"); !bytes.Equal(got, []byte("earlier")) {
		t.Fatalf("old = %q; want it left alone", got)
	}
}

func TestAdoptRenamedSecretWithNothingToMove(t *testing.T) {
	kc := fake.NewKeychain()
	if err := platform.AdoptRenamedSecret(kc, "old", "new", "acct"); err != nil {
		t.Fatal(err)
	}
	if _, err := kc.Get("new", "acct"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("new = %v; want nothing stored", err)
	}
}
