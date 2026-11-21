package child

import (
	"encoding/json"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteStorage is Storage backed by the host, which namespaces every key to this extension.
type remoteStorage struct {
	link *hostLink
	memo memoCore
}

func (s remoteStorage) Get(key string, into any) (bool, error) {
	result, err := readEntry(s.link, protocol.ServiceStorage, key)
	if err != nil || !result.Found {
		return false, err
	}
	return true, decodeEntry(result.Value, into)
}

func (s remoteStorage) Set(key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return api.ErrInvalidInput
	}
	return s.link.Call(protocol.ServiceStorage, protocol.OpSet, protocol.KeyValueArgs{Key: key, Value: raw}, nil)
}

func (s remoteStorage) Delete(key string) error {
	return s.link.Call(protocol.ServiceStorage, protocol.OpDelete, protocol.KeyArgs{Key: key}, nil)
}

func (s remoteStorage) Clear() error {
	return s.link.Call(protocol.ServiceStorage, protocol.OpClear, nil, nil)
}

func (s remoteStorage) Memo(key string, ttl time.Duration, into any, load func() (any, error)) error {
	return s.memo.Memo(key, ttl, into, load)
}

func (s remoteStorage) MemoStale(key string, ttl, staleTTL time.Duration, into any, load func() (any, error)) error {
	return s.memo.MemoStale(key, ttl, staleTTL, into, load)
}

// storageKV adapts the storage service to the key-value view Memo needs.
type storageKV struct {
	link *hostLink
}

func (k storageKV) read(key string) (json.RawMessage, bool, error) {
	result, err := readEntry(k.link, protocol.ServiceStorage, key)
	if err != nil {
		return nil, false, err
	}
	return result.Value, result.Found, nil
}

func (k storageKV) write(key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return api.ErrInvalidInput
	}
	return k.link.Call(protocol.ServiceStorage, protocol.OpSet, protocol.KeyValueArgs{Key: key, Value: raw}, nil)
}

// readEntry asks a keyed service whether an entry exists and returns its JSON.
func readEntry(link *hostLink, service, key string) (protocol.GetResult, error) {
	var result protocol.GetResult
	err := link.Call(service, protocol.OpGet, protocol.KeyArgs{Key: key}, &result)
	return result, err
}

// decodeEntry decodes a stored JSON value into the caller's pointer. A nil pointer discards the value.
func decodeEntry(raw json.RawMessage, into any) error {
	if into == nil {
		return nil
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return api.ErrInvalidInput
	}
	return nil
}
