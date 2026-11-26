package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	maxAITools                = 32
	maxAIToolDescriptionRunes = 400
	maxAIToolInputBytes       = 8 << 10
)

var aiToolNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// aiToolClassRank orders the classes from the one that asks least to the ones that ask every time.
var aiToolClassRank = map[AIToolClass]int{
	AIToolRead: 0, AIToolWrite: 1, AIToolSensitiveRead: 2, AIToolDestructive: 3, AIToolExternal: 3, AIToolSetting: 3,
}

// EnforcedAIToolClass is the class the host runs a declared tool with: its declared class raised to the extension's
// floor. A folder extension cannot make its tools run unasked, and neither can an extension that reads secrets.
func (m Manifest) EnforcedAIToolClass(tool AITool) AIToolClass {
	floor := AIToolRead
	if !m.IsBuiltIn() || m.declaresSensitiveData() {
		floor = AIToolSensitiveRead
	}
	if aiToolClassRank[tool.Class] < aiToolClassRank[floor] {
		return floor
	}
	return tool.Class
}

func (m Manifest) declaresSensitiveData() bool {
	for _, capability := range m.Capabilities {
		if capability == CapabilityClipboardHistory || capability == CapabilitySecrets {
			return true
		}
	}
	return false
}

func validateAITools(tools []AITool) error {
	if len(tools) > maxAITools {
		return fmt.Errorf("an extension declares at most %d AI tools", maxAITools)
	}
	seen := map[string]bool{}
	for _, tool := range tools {
		if !aiToolNamePattern.MatchString(tool.Name) {
			return fmt.Errorf("AI tool name %q must be lowercase words joined by -, such as search-snippets", tool.Name)
		}
		if seen[tool.Name] {
			return fmt.Errorf("AI tool %q is declared twice", tool.Name)
		}
		seen[tool.Name] = true
		if err := validateAITool(tool); err != nil {
			return fmt.Errorf("AI tool %q: %w", tool.Name, err)
		}
	}
	return nil
}

func validateAITool(tool AITool) error {
	if strings.TrimSpace(tool.Description) == "" || len([]rune(tool.Description)) > maxAIToolDescriptionRunes {
		return fmt.Errorf("the description is required and at most %d characters", maxAIToolDescriptionRunes)
	}
	if _, known := aiToolClassRank[tool.Class]; !known {
		return fmt.Errorf("class %q is not one of read, write, sensitive-read, destructive, external or setting", tool.Class)
	}
	return validateAIToolInput(tool.InputSchema)
}

// validateAIToolInput accepts no schema, or a JSON Schema whose type is object.
func validateAIToolInput(input json.RawMessage) error {
	if len(input) == 0 {
		return nil
	}
	if len(input) > maxAIToolInputBytes {
		return fmt.Errorf("the input schema is larger than %d bytes", maxAIToolInputBytes)
	}
	var schema struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(input, &schema); err != nil || schema.Type != "object" {
		return fmt.Errorf("the input schema must be a JSON Schema object with \"type\": \"object\"")
	}
	return nil
}
