package api

import (
	"fmt"
	"strings"
)

// Operation names a host intent whose structural support can be required by opaque extension code.
// Requirements are all-of discovery metadata, never permission grants. Omit them for conditional
// fallback code. Unknown operations are unsupported; permission and login readiness are not support.
// Available since API 1.1. Declare APIVersion 1.1 when relying on Requires.
type Operation string

const (
	OperationFilePreview   Operation = "files.preview"
	OperationSwitchDesktop Operation = "windows.switch-desktop"
)

func validateRequirements(required []Operation) error {
	if len(required) > 32 {
		return fmt.Errorf("requires is limited to 32 operations")
	}
	for _, operation := range required {
		if strings.TrimSpace(string(operation)) == "" {
			return fmt.Errorf("requires needs a non-empty operation name")
		}
	}
	return nil
}
