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

func TestFingerprintKeyAdoptsTheKeyKeptUnderTheEarlierName(t *testing.T) {
	kc := fake.NewKeychain()
	expected := bytes.Repeat([]byte{7}, 32)
	_ = kc.Set(renamedKeyService, keyAccount, expected)
	key, err := FingerprintKey(kc, true, bytes.NewReader(nil))
	if err != nil || !bytes.Equal(key, expected) {
		t.Fatalf("key = %x, %v; want the key kept under the earlier name", key, err)
	}
	if _, err := kc.Get(renamedKeyService, keyAccount); err == nil {
		t.Fatal("the earlier entry was kept")
	}
}
