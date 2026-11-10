package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// servicesRig is what one extension runs on: the host ports the services use and a data root they share.
type servicesRig struct {
	deps services.Deps
	root string
}

func newServicesRig(t *testing.T) servicesRig {
	t.Helper()
	ports := real.NewPorts()
	logs, err := logx.New(ports.FS, ports.Clock, t.TempDir(), logx.Options{})
	if err != nil {
		t.Fatalf("logx: %v", err)
	}
	t.Cleanup(func() { _ = logs.Close() })
	root := t.TempDir()
	return servicesRig{
		root: root,
		deps: services.Deps{
			Files:    ports.FS,
			Clock:    ports.Clock,
			DataRoot: root,
			Keychain: fake.NewKeychain(),
			Network:  ports.PublicHTTP,
			Logs:     logs,
		},
	}
}

// servicedRuntime starts the fixture named name with the services of manifest on top of the rig. Budgets are wide
// because the fixtures make many round trips to the host.
func servicedRuntime(t *testing.T, name string, rig servicesRig, manifest api.Manifest) *Runtime {
	return servicedRuntimeWithWorker(t, name, rig, manifest, real.NewWorker())
}

func servicedRuntimeWithWorker(t *testing.T, name string, rig servicesRig, manifest api.Manifest, worker platform.Worker) *Runtime {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	handler := services.New(rig.deps, manifest)
	t.Cleanup(func() { _ = handler.Close() })
	cfg := Config{
		Worker:       worker,
		Clock:        real.NewPorts().Clock,
		Executable:   exe,
		StartTimeout: 10 * time.Second,
		Budgets: map[string]Budget{
			protocol.MethodSearch: {Hard: 30 * time.Second},
			protocol.MethodLoad:   {Hard: 30 * time.Second},
		},
	}
	runtime := New(cfg, manifest.Identity, fixture(t, name), handler)
	t.Cleanup(runtime.Stop)
	startAndWait(t, runtime)
	return runtime
}

func fixtureManifest(id string, capabilities ...api.Capability) api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: id, Title: id}, Capabilities: capabilities}
}

// rowTitle runs one search query and returns the title of its only row.
func rowTitle(t *testing.T, r *Runtime, query string) string {
	t.Helper()
	response, err := callSearch(t, r, query)
	if err != nil {
		t.Fatalf("call %q: %v", query, err)
	}
	if response.Failure != nil {
		t.Fatalf("call %q failed: %+v", query, *response.Failure)
	}
	var items []api.Item
	if err := json.Unmarshal(response.Data, &items); err != nil || len(items) != 1 {
		t.Fatalf("call %q returned %s (%v); want one row", query, response.Data, err)
	}
	return items[0].Title
}

// dataFiles returns the text of every file under root, for leak checks.
func dataFiles(t *testing.T, root string) string {
	t.Helper()
	var text strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		text.Write(data)
		return readErr
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return text.String()
}

func allServiceCapabilities() []api.Capability {
	return []api.Capability{api.CapabilitySecrets, api.CapabilitySettingsWrite, api.CapabilityNetwork}
}

func servicesManifest() api.Manifest {
	manifest := fixtureManifest("acme.services", allServiceCapabilities()...)
	manifest.Settings = []api.SettingDefinition{
		{ID: "units", Type: api.SettingString, Value: []byte(`"metric"`)},
		{ID: "compact", Type: api.SettingBoolean, Value: []byte(`false`)},
	}
	return manifest
}

func TestEveryServiceAnswersThroughTheChildProcess(t *testing.T) {
	requireUnixProcesses(t)
	rig := newServicesRig(t)
	r := servicedRuntime(t, "acme.services", rig, servicesManifest())
	want := map[string]string{
		"storage":  "storage found=true city=Lisbon",
		"memo":     "memo value=42 again=42 loads=1",
		"secrets":  "secrets found=true value=canary-token-7f3a9c",
		"settings": "settings default=metric definitions=2 compact=true",
		"logs":     "logs recent=1",
		"text":     "text=hi there cursor=8",
		"network":  "network permission=true",
		"cache":    "cache found=true value=7",
		"shared":   "shared written",
	}
	for query, title := range want {
		if got := rowTitle(t, r, query); got != title {
			t.Fatalf("%s row = %q; want %q", query, got, title)
		}
	}
	if strings.Contains(dataFiles(t, rig.root), "canary-token-7f3a9c") {
		t.Fatal("the secret value reached the data folder")
	}
}

func TestDataLoaderRunsInTheChildAndCanBeCached(t *testing.T) {
	requireUnixProcesses(t)
	rig := newServicesRig(t)
	r := servicedRuntime(t, "acme.services", rig, servicesManifest())
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	items, err := r.Load(ctx, "feed")
	if err != nil || len(items) != 1 || items[0].Title != "loaded feed" {
		t.Fatalf("load = %v (%v); want the loader's item", items, err)
	}
	_, err = r.Load(ctx, "fail")
	var failure *CallError
	if err == nil || !errors.As(err, &failure) || !strings.Contains(failure.Failure.Message, "feed fail is down") {
		t.Fatalf("failing loader error = %v; want the loader's message as a call error", err)
	}
}

func TestHostileExtensionHitsEveryLimitAndKeepsAnswering(t *testing.T) {
	requireUnixProcesses(t)
	rig := newServicesRig(t)
	rig.deps.Clock = fake.NewClock()
	r := servicedRuntime(t, "acme.hostile", rig, fixtureManifest("acme.hostile"))
	want := map[string]string{
		"quota":      "quota written=1000 quota=true",
		"big":        "big quota=true",
		"traversal":  "traversal found=false err=false",
		"undeclared": "undeclared permission=true",
		"private":    "private permission=true",
		"settings":   "settings permission=true",
		"flood":      "flood kept=300",
	}
	for query, title := range want {
		if got := rowTitle(t, r, query); got != title {
			t.Fatalf("%s row = %q; want %q", query, got, title)
		}
	}
	if state := r.Status().State; state != StateRunning {
		t.Fatalf("hostile calls left the extension %s (%s); want running", state, r.Status().Reason)
	}
}

func TestExtensionCannotReadAnotherExtensionsStorage(t *testing.T) {
	requireUnixProcesses(t)
	rig := newServicesRig(t)
	writer := servicedRuntime(t, "acme.services", rig, servicesManifest())
	if got := rowTitle(t, writer, "shared"); got != "shared written" {
		t.Fatalf("writer row = %q", got)
	}
	reader := servicedRuntime(t, "acme.hostile", rig, fixtureManifest("acme.hostile"))
	if got := rowTitle(t, reader, "read-shared"); got != "read-shared found=false err=false" {
		t.Fatalf("reader row = %q; want the other extension's key to be absent", got)
	}
}
