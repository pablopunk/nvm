package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

const (
	secretsFile         = "secrets-index.json"
	secretServicePrefix = "nevermind.extension."
	maxSecretNames      = 64
	maxSecretValueBytes = 16 << 10
)

var secretNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]{0,127}$`)

// secretIndexDoc lists the names of the extension's secrets, never their values. Clear needs the list because the
// keychain cannot enumerate one service's accounts.
type secretIndexDoc struct {
	Names []string `json:"names"`
}

func newSecretIndex() secretIndexDoc {
	return secretIndexDoc{}
}

// serviceFor is the keychain service of one extension. Its secrets never share a service with another extension.
func (h *Handler) serviceFor() string {
	return secretServicePrefix + h.manifest.ID
}

func (h *Handler) serveSecrets(call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilitySecrets); err != nil {
		return nil, err
	}
	if h.deps.Keychain == nil {
		return nil, unavailable("the OS keychain is not available; secrets are off, and nothing is stored in a file instead")
	}
	h.secretsMu.Lock()
	defer h.secretsMu.Unlock()
	switch call.Op {
	case protocol.OpGet:
		return h.secretGet(call.Args)
	case protocol.OpSet:
		return nil, h.secretSet(call.Args)
	case protocol.OpDelete:
		return nil, h.secretDelete(call.Args)
	case protocol.OpClear:
		return nil, h.secretClear()
	}
	return nil, invalid("secrets has no operation %q", call.Op)
}

func (h *Handler) secretGet(args json.RawMessage) (protocol.GetResult, error) {
	var request protocol.KeyArgs
	if err := decodeArgs(args, &request); err != nil {
		return protocol.GetResult{}, err
	}
	if err := checkSecretName(request.Key); err != nil {
		return protocol.GetResult{}, err
	}
	value, err := h.deps.Keychain.Get(h.serviceFor(), request.Key)
	if errors.Is(err, platform.ErrNotFound) {
		return protocol.GetResult{}, nil
	}
	if err != nil {
		return protocol.GetResult{}, keychainFailure(err)
	}
	raw, _ := json.Marshal(string(value))
	return protocol.GetResult{Found: true, Value: raw}, nil
}

func (h *Handler) secretSet(args json.RawMessage) error {
	var request protocol.KeyValueArgs
	if err := decodeArgs(args, &request); err != nil {
		return err
	}
	if err := checkSecretName(request.Key); err != nil {
		return err
	}
	var value string
	if err := json.Unmarshal(request.Value, &value); err != nil {
		return invalid("a secret value must be a string")
	}
	if len(value) > maxSecretValueBytes {
		return quota(fmt.Errorf("a secret value may be at most %d KiB", maxSecretValueBytes>>10))
	}
	docs, err := h.openedDocuments()
	if err != nil {
		return err
	}
	index, err := docs.secrets.Load()
	if err != nil {
		return unavailable("the secret index cannot be read")
	}
	if !slices.Contains(index.Names, request.Key) && len(index.Names) >= maxSecretNames {
		return quota(fmt.Errorf("an extension may keep at most %d secrets", maxSecretNames))
	}
	if err := h.deps.Keychain.Set(h.serviceFor(), request.Key, []byte(value)); err != nil {
		return keychainFailure(err)
	}
	return rememberSecretName(docs, request.Key)
}

func (h *Handler) secretDelete(args json.RawMessage) error {
	var request protocol.KeyArgs
	if err := decodeArgs(args, &request); err != nil {
		return err
	}
	if err := checkSecretName(request.Key); err != nil {
		return err
	}
	if err := h.deps.Keychain.Delete(h.serviceFor(), request.Key); err != nil {
		return keychainFailure(err)
	}
	docs, err := h.openedDocuments()
	if err != nil {
		return err
	}
	return forgetSecretName(docs, request.Key)
}

// secretClear deletes every secret the index names. A failed deletion keeps its name in the index, so a retry
// still finds it.
func (h *Handler) secretClear() error {
	docs, err := h.openedDocuments()
	if err != nil {
		return err
	}
	index, err := docs.secrets.Load()
	if err != nil {
		return unavailable("the secret index cannot be read")
	}
	for _, name := range index.Names {
		if err := h.deps.Keychain.Delete(h.serviceFor(), name); err != nil {
			return keychainFailure(err)
		}
		if err := forgetSecretName(docs, name); err != nil {
			return err
		}
	}
	return nil
}

func rememberSecretName(docs *documents, name string) error {
	return secretIndexUpdate(docs, func(index *secretIndexDoc) {
		if !slices.Contains(index.Names, name) {
			index.Names = append(index.Names, name)
		}
	})
}

func forgetSecretName(docs *documents, name string) error {
	return secretIndexUpdate(docs, func(index *secretIndexDoc) {
		index.Names = slices.DeleteFunc(index.Names, func(candidate string) bool { return candidate == name })
	})
}

// secretIndexUpdate changes and saves the index.
func secretIndexUpdate(docs *documents, change func(*secretIndexDoc)) error {
	if err := docs.secrets.Update(change); err != nil {
		return unavailable("the secret index cannot be saved: %v", err)
	}
	if err := docs.secrets.Save(); err != nil {
		return unavailable("the secret index cannot be saved: %v", err)
	}
	return nil
}

func checkSecretName(name string) error {
	if !secretNamePattern.MatchString(name) {
		return invalid("a secret name uses letters, digits, '_', '.' or '-', starts with a letter or '_', and is at most 128 characters")
	}
	return nil
}

// keychainFailure explains a keychain error without its content. An unsupported keychain is unavailable, not a
// reason to store the value somewhere else.
func keychainFailure(err error) error {
	if errors.Is(err, platform.ErrUnsupported) {
		return unavailable("the OS keychain is not available; secrets are off, and nothing is stored in a file instead")
	}
	return unavailable("the OS keychain refused the request")
}
