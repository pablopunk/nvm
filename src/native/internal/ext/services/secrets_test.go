package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
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
