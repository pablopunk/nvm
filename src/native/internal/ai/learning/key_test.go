package learning

import (
	"bytes"
	"io/fs"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestFingerprintKeyPersistsAndFailsClosed(t *testing.T) {
	kc := fake.NewKeychain()
	expected := bytes.Repeat([]byte{42}, 32)
	key, err := FingerprintKey(kc, false, bytes.NewReader(expected))
	if err != nil || !bytes.Equal(key, expected) {
		t.Fatalf("new key: %v", err)
	}
	key[0] = 0
	key, err = FingerprintKey(kc, true, bytes.NewReader(nil))
	if err != nil || !bytes.Equal(key, expected) {
		t.Fatal("key was not persisted independently")
	}
	missing := fake.NewKeychain()
	if _, err := FingerprintKey(missing, true, bytes.NewReader(expected)); err == nil {
		t.Fatal("replaced missing key for existing rules")
	}
	broken := fake.NewKeychain()
	broken.FailNext(fake.OpKeychainGet, fs.ErrPermission)
	if _, err := FingerprintKey(broken, false, bytes.NewReader(expected)); err == nil {
		t.Fatal("ignored keychain failure")
	}
}
