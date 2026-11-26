package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func manifestWithTools(tools ...AITool) Manifest {
	return Manifest{APIVersion: "1.0", Identity: Identity{ID: "acme.weather", Title: "Weather"}, AITools: tools}
}

func TestManifestRefusesMalformedAITools(t *testing.T) {
	valid := AITool{Name: "forecast", Description: "Today's forecast.", Class: AIToolRead}
	withName := func(name string) AITool { tool := valid; tool.Name = name; return tool }
	cases := map[string]struct {
		tools  []AITool
		reason string
	}{
		"bad name":       {[]AITool{withName("Forecast Now")}, "lowercase words"},
		"declared twice": {[]AITool{valid, valid}, "declared twice"},
		"no description": {[]AITool{{Name: "forecast", Class: AIToolRead}}, "description is required"},
		"unknown class":  {[]AITool{{Name: "forecast", Description: "x", Class: "harmless"}}, "is not one of"},
		"bad schema":     {[]AITool{{Name: "forecast", Description: "x", Class: AIToolRead, InputSchema: json.RawMessage(`{"type":"string"}`)}}, "JSON Schema object"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := manifestWithTools(c.tools...).Validate()
			if err == nil || !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("err = %v; want %q", err, c.reason)
			}
		})
	}
	if err := manifestWithTools(valid).Validate(); err != nil {
		t.Fatalf("a valid tool was refused: %v", err)
	}
}

// The host raises a declared class to the extension's floor and never lowers it: a folder extension's tools, and the
// tools of one that reads secrets or the clipboard history, cannot run unasked.
func TestTheEnforcedClassIsNeverBelowTheExtensionsFloor(t *testing.T) {
	builtIn := Manifest{Identity: Identity{ID: "nevermind.snippets"}}
	sensitiveBuiltIn := Manifest{Identity: Identity{ID: "nevermind.clipboard"}, Capabilities: []Capability{CapabilityClipboardHistory}}
	folder := Manifest{Identity: Identity{ID: "acme.weather"}}
	cases := []struct {
		manifest Manifest
		declared AIToolClass
		want     AIToolClass
	}{
		{builtIn, AIToolRead, AIToolRead},
		{builtIn, AIToolWrite, AIToolWrite},
		{sensitiveBuiltIn, AIToolRead, AIToolSensitiveRead},
		{sensitiveBuiltIn, AIToolWrite, AIToolSensitiveRead},
		{folder, AIToolRead, AIToolSensitiveRead},
		{folder, AIToolWrite, AIToolSensitiveRead},
		{folder, AIToolDestructive, AIToolDestructive},
		{folder, AIToolExternal, AIToolExternal},
		{builtIn, AIToolSetting, AIToolSetting},
	}
	for _, c := range cases {
		if got := c.manifest.EnforcedAIToolClass(AITool{Class: c.declared}); got != c.want {
			t.Errorf("%s declaring %s runs as %s; want %s", c.manifest.ID, c.declared, got, c.want)
		}
	}
}
