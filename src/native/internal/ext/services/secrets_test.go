package services

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
)

const canary = "canary-token-7f3a9c"

func secretsManifest(id string) api.Manifest {
	return manifestFor(id, api.CapabilitySecrets)
}

func TestSecretValuesGoOnlyToTheKeychain(t *testing.T) {
	r := newRig(t)
	h := handlerFor(t, r, secretsManifest("acme.one"))
	mustOK(t, serve(t, h, protocol.ServiceSecrets, protocol.OpSet, protocol.KeyValueArgs{Key: "API_TOKEN", Value: []byte(`"` + canary + `"`)}))
	if !strings.Contains(string(mustOK(t, serve(t, h, protocol.ServiceSecrets, protocol.OpGet, protocol.KeyArgs{Key: "API_TOKEN"}))), canary) {
		t.Fatal("the secret did not come back")
	}
	value, err := r.keychain.Get("nevermind.extension.acme.one", "API_TOKEN")
	if err != nil || string(value) != canary {
		t.Fatalf("keychain value = %q (%v); want the canary under the extension's service", value, err)
	}
	if strings.Contains(dataFiles(t, r.root), canary) {
		t.Fatal("the secret value reached a file in the data folder")
	}
}

func TestSecretsAreNamespacedPerExtension(t *testing.T) {
	r := newRig(t)
	first := handlerFor(t, r, secretsManifest("acme.one"))
	second := handlerFor(t, r, secretsManifest("acme.two"))
	mustOK(t, serve(t, first, protocol.ServiceSecrets, protocol.OpSet, protocol.KeyValueArgs{Key: "TOKEN", Value: []byte(`"one"`)}))
	if decodeGet(t, mustOK(t, serve(t, second, protocol.ServiceSecrets, protocol.OpGet, protocol.KeyArgs{Key: "TOKEN"}))).Found {
		t.Fatal("another extension read the secret")
	}
}

func TestMissingKeychainIsUnavailableAndNothingIsWrittenToAFile(t *testing.T) {
	r := newRig(t)
	r.keychain.FailNext(fake.OpKeychainSet, platform.ErrUnsupported)
	h := handlerFor(t, r, secretsManifest("acme.one"))
	reply := serve(t, h, protocol.ServiceSecrets, protocol.OpSet, protocol.KeyValueArgs{Key: "TOKEN", Value: []byte(`"` + canary + `"`)})
	requireCode(t, reply, api.CodeUnavailable, "keychain")
	if strings.Contains(dataFiles(t, r.root), canary) {
		t.Fatal("secrets fell back to a file when the keychain was unavailable")
	}
}

func TestClearRemovesEverySecretTheIndexNames(t *testing.T) {
	r := newRig(t)
	h := handlerFor(t, r, secretsManifest("acme.one"))
	for _, name := range []string{"A", "B", "C"} {
		mustOK(t, serve(t, h, protocol.ServiceSecrets, protocol.OpSet, protocol.KeyValueArgs{Key: name, Value: []byte(`"v"`)}))
	}
	mustOK(t, serve(t, h, protocol.ServiceSecrets, protocol.OpClear, nil))
	for _, name := range []string{"A", "B", "C"} {
		if _, err := r.keychain.Get("nevermind.extension.acme.one", name); !errors.Is(err, platform.ErrNotFound) {
			t.Fatalf("secret %s survived clear: %v", name, err)
		}
	}
}

func TestSecretNameAndCountQuotas(t *testing.T) {
	r := newRig(t)
	h := handlerFor(t, r, secretsManifest("acme.one"))
	requireCode(t, serve(t, h, protocol.ServiceSecrets, protocol.OpSet, protocol.KeyValueArgs{Key: "bad name", Value: []byte(`"v"`)}), api.CodeInvalid, "")
	for i := 0; i < maxSecretNames; i++ {
		mustOK(t, serve(t, h, protocol.ServiceSecrets, protocol.OpSet, protocol.KeyValueArgs{Key: keyName(i), Value: []byte(`"v"`)}))
	}
	requireCode(t, serve(t, h, protocol.ServiceSecrets, protocol.OpSet, protocol.KeyValueArgs{Key: "extra", Value: []byte(`"v"`)}), api.CodeQuota, "secrets")
}

// countingKeychain records calls even when the underlying operation fails.
type countingKeychain struct {
	*fake.Keychain
	sets, deletes int
}

func (k *countingKeychain) Set(service, account string, value []byte) error {
	k.sets++
	return k.Keychain.Set(service, account, value)
}

func (k *countingKeychain) Delete(service, account string) error {
	k.deletes++
	return k.Keychain.Delete(service, account)
}

func TestUnreadableSecretIndexBlocksKeychainChanges(t *testing.T) {
	for _, op := range []string{protocol.OpSet, protocol.OpDelete, protocol.OpClear} {
		t.Run(op, func(t *testing.T) {
			files := fake.NewFS()
			clock := fake.NewClock()
			keychain := &countingKeychain{Keychain: fake.NewKeychain()}
			deps := Deps{Files: files, Clock: clock, DataRoot: testpath.Abs("data"), Keychain: keychain}
			first := New(deps, secretsManifest("acme.one"))
			mustOK(t, serve(t, first, protocol.ServiceSecrets, protocol.OpSet, protocol.KeyValueArgs{Key: "TOKEN", Value: []byte(`"original"`)}))
			if err := first.Close(); err != nil {
				t.Fatal(err)
			}
			h := New(deps, secretsManifest("acme.one"))
			t.Cleanup(func() { _ = h.Close() })
			docs, err := h.openedDocuments()
			if err != nil {
				t.Fatal(err)
			}
			path := docs.secrets.Path()
			before, err := files.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			keychain.sets, keychain.deletes = 0, 0
			files.FailNext(fake.OpReadFile, errors.New("private disk detail"))
			var args any
			if op == protocol.OpSet {
				args = protocol.KeyValueArgs{Key: "TOKEN", Value: []byte(`"replacement"`)}
			} else if op == protocol.OpDelete {
				args = protocol.KeyArgs{Key: "TOKEN"}
			}
			reply := serve(t, h, protocol.ServiceSecrets, op, args)
			requireCode(t, reply, api.CodeUnavailable, "secret index")
			if strings.Contains(reply.Failure.Message, "private disk detail") {
				t.Fatal("read error leaked")
			}
			if keychain.sets != 0 || keychain.deletes != 0 {
				t.Fatalf("keychain calls: set=%d delete=%d", keychain.sets, keychain.deletes)
			}
			value, err := keychain.Get("nevermind.extension.acme.one", "TOKEN")
			if err != nil || string(value) != "original" {
				t.Fatalf("secret changed: %q, %v", value, err)
			}
			after, err := files.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("index changed: %v", err)
			}
		})
	}
}
