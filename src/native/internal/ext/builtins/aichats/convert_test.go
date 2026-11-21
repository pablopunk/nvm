package aichats

import (
	"strings"
	"testing"
)

func TestConversionPromptCarriesTheSourceAndTheMappingTool(t *testing.T) {
	prompt := conversionPrompt("old.ts", "export default {}")
	for _, want := range []string{"read_1x_mapping", `"old.ts"`, "export default {}"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
}
