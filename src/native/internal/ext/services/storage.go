package services

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

const storageFile = "storage.json"

// Storage quotas per extension. They bound what one extension keeps on disk; the store writes the whole file, so
// the total is also the cost of every write.
const (
	maxStorageKeys       = 1000
	maxStorageKeyBytes   = 256
	maxStorageValueBytes = 256 << 10
	maxStorageBytes      = 4 << 20
)

// storageDoc is the file behind Storage: one JSON value per key.
type storageDoc struct {
	Values map[string]json.RawMessage `json:"values"`
}

func newStorageDoc() storageDoc {
	return storageDoc{Values: map[string]json.RawMessage{}}
}

func (h *Handler) serveStorage(call protocol.ServiceCall) (any, error) {
	docs, err := h.openedDocuments()
	if err != nil {
		return nil, err
	}
	switch call.Op {
	case protocol.OpGet:
		return h.storageGet(docs, call.Args)
	case protocol.OpSet:
		return nil, h.storageSet(docs, call.Args)
	case protocol.OpDelete:
		return nil, h.storageDelete(docs, call.Args)
	case protocol.OpClear:
		return nil, storageUpdate(docs, func(doc *storageDoc) error {
			doc.Values = map[string]json.RawMessage{}
			return nil
		})
	}
	return nil, invalid("storage has no operation %q", call.Op)
}

func (h *Handler) storageGet(docs *documents, args json.RawMessage) (protocol.GetResult, error) {
	var request protocol.KeyArgs
	if err := decodeArgs(args, &request); err != nil {
		return protocol.GetResult{}, err
	}
	doc, err := docs.storage.Load()
	if err != nil {
		return protocol.GetResult{}, unavailable("the extension's storage cannot be read: %v", err)
	}
	value, found := doc.Values[request.Key]
	return protocol.GetResult{Found: found, Value: value}, nil
}

func (h *Handler) storageSet(docs *documents, args json.RawMessage) error {
	var request protocol.KeyValueArgs
	if err := decodeArgs(args, &request); err != nil {
		return err
	}
	if err := checkStorageKey(request.Key); err != nil {
		return err
	}
	if len(request.Value) > maxStorageValueBytes || !json.Valid(request.Value) {
		return quota(fmt.Errorf("a storage value must be valid JSON of at most %d KiB", maxStorageValueBytes>>10))
	}
	return storageUpdate(docs, func(doc *storageDoc) error {
		if err := checkStorageRoom(*doc, request.Key, len(request.Value)); err != nil {
			return err
		}
		doc.Values[request.Key] = request.Value
		return nil
	})
}

func (h *Handler) storageDelete(docs *documents, args json.RawMessage) error {
	var request protocol.KeyArgs
	if err := decodeArgs(args, &request); err != nil {
		return err
	}
	return storageUpdate(docs, func(doc *storageDoc) error {
		delete(doc.Values, request.Key)
		return nil
	})
}

// storageUpdate runs change under the document's lock. A change that returns an error leaves the document as it
// was, so a refused write never reaches the file.
func storageUpdate(docs *documents, change func(doc *storageDoc) error) error {
	var refused error
	err := docs.storage.Update(func(doc *storageDoc) {
		refused = change(doc)
	})
	if err != nil {
		return unavailable("the extension's storage cannot take changes: %v", err)
	}
	return refused
}

// checkStorageRoom reports whether key can take a value of valueBytes without breaking the count or size quota.
func checkStorageRoom(doc storageDoc, key string, valueBytes int) error {
	if _, exists := doc.Values[key]; !exists && len(doc.Values) >= maxStorageKeys {
		return quota(fmt.Errorf("an extension may keep at most %d storage keys", maxStorageKeys))
	}
	after := storageBytes(doc) - len(doc.Values[key]) + valueBytes + len(key)
	if after > maxStorageBytes {
		return quota(fmt.Errorf("the extension's storage would exceed %d MiB", maxStorageBytes>>20))
	}
	return nil
}

func storageBytes(doc storageDoc) int {
	total := 0
	for key, value := range doc.Values {
		total += len(key) + len(value)
	}
	return total
}

func checkStorageKey(key string) error {
	if key == "" || len(key) > maxStorageKeyBytes || !utf8.ValidString(key) {
		return invalid("a storage key must be non-empty UTF-8 of at most %d bytes", maxStorageKeyBytes)
	}
	return nil
}

// quota wraps a quota cause so the child sees api.ErrQuotaExceeded.
func quota(cause error) error {
	return fmt.Errorf("%w: %v", api.ErrQuotaExceeded, cause)
}
