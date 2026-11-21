//go:build automation

package app

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// The rig's recorder answers for every system intent the app knows, so the palette offers the commands on any image.
func TestRigPortsReportEverySystemCapabilityItRecords(t *testing.T) {
	ports := rigPorts(fake.NewSet().Ports())
	for _, capability := range platform.AllCapabilities() {
		if strings.HasPrefix(string(capability), "system.") && !ports.Capabilities.Supported(capability) {
			t.Errorf("the rig records %s but does not report it", capability)
		}
	}
}
