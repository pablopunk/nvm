package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// rig is the set of fake ports the services run on, plus the data root they share.
type rig struct {
	clock     *fake.Clock
	keychain  *fake.Keychain
	network   *fake.HTTP
	clipboard *fake.Clipboard
	logs      *logx.Logger
	root      string
}

func newRig(t *testing.T) rig {
	t.Helper()
	ports := real.NewPorts()
	logs, err := logx.New(ports.FS, ports.Clock, t.TempDir(), logx.Options{})
	if err != nil {
		t.Fatalf("logx: %v", err)
	}
	t.Cleanup(func() { _ = logs.Close() })
	return rig{
		clock:     fake.NewClock(),
		keychain:  fake.NewKeychain(),
		network:   fake.NewHTTP(),
		clipboard: fake.NewClipboard(),
		logs:      logs,
		root:      t.TempDir(),
	}
}

// manifestFor builds the manifest of one test extension. The id is the namespace under test.
func manifestFor(id string, capabilities ...api.Capability) api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: id, Title: id},
		Capabilities: capabilities,
	}
}

// handlerFor opens the services of one extension over the rig and closes them with the test.
func handlerFor(t *testing.T, r rig, manifest api.Manifest) *Handler {
	t.Helper()
	ports := real.NewPorts()
	handler := New(Deps{
		Files:     ports.FS,
		Clock:     r.clock,
		DataRoot:  r.root,
		Keychain:  r.keychain,
		Network:   r.network,
		Clipboard: r.clipboard,
		Logs:      r.logs,
	}, manifest)
	t.Cleanup(func() { _ = handler.Close() })
	return handler
}

// serve sends one call with JSON arguments and returns the reply.
func serve(t *testing.T, h *Handler, service, op string, args any) protocol.ServiceReply {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return h.Serve(context.Background(), protocol.ServiceCall{CallID: 1, Service: service, Op: op, Args: raw})
}

// mustOK fails the test unless the reply succeeded, and returns its data.
func mustOK(t *testing.T, reply protocol.ServiceReply) json.RawMessage {
	t.Helper()
	if reply.Failure != nil {
		t.Fatalf("reply failed: %+v", *reply.Failure)
	}
	return reply.Data
}

// failureCode fails the test unless the reply is a failure, and returns it.
func failureCode(t *testing.T, reply protocol.ServiceReply) api.Failure {
	t.Helper()
	if reply.Failure == nil {
		t.Fatalf("reply succeeded with data %s; want a failure", reply.Data)
	}
	return *reply.Failure
}

// requireCode checks a failure's code and that its message names what the caller needs to know.
func requireCode(t *testing.T, reply protocol.ServiceReply, code api.ErrorCode, mentions string) {
	t.Helper()
	failure := failureCode(t, reply)
	if failure.Code != code {
		t.Fatalf("code = %s (%s); want %s", failure.Code, failure.Message, code)
	}
	if mentions != "" && !strings.Contains(failure.Message, mentions) {
		t.Fatalf("message %q does not mention %q", failure.Message, mentions)
	}
}

// dataFiles returns the text of every file under the data root, for leak checks.
func dataFiles(t *testing.T, root string) string {
	t.Helper()
	var text strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text.Write(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk data root: %v", err)
	}
	return text.String()
}

// decodeGet reads a GetResult from reply data.
func decodeGet(t *testing.T, data json.RawMessage) protocol.GetResult {
	t.Helper()
	var result protocol.GetResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode get result %s: %v", data, err)
	}
	return result
}

// openedDocumentsForTest exposes the typed documents to tests in this package.
func (h *Handler) openedDocumentsForTest() (*documents, error) {
	return h.openedDocuments()
}

// jsonInto decodes reply data into v.
func jsonInto(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
