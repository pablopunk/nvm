package manager

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"testing"
)

func TestNetworkAdmissionRequirementUsesPublicPortWithoutReadinessProbe(t *testing.T) {
	manager := &Manager{}
	if manager.OperationsSupported([]api.Operation{api.OperationNetworkFetch}) {
		t.Fatal("missing port advertised")
	}
	port := fake.NewHTTP()
	manager.serviceDeps.Network = port
	if !manager.OperationsSupported([]api.Operation{api.OperationNetworkFetch}) || len(port.Requests()) != 0 {
		t.Fatal("requirements differ from wired port or probed network")
	}
}
