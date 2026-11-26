package websearch

import "testing"

func TestEngineSettingDeclaresEveryEngine(t *testing.T) {
	manifest := New("bing").Manifest()
	if err := manifest.ValidateBuiltIn(); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
	definition := manifest.Settings[0]
	if len(definition.Choices) != len(engines) || definition.Choices[3].Value != "brave" || definition.Choices[3].Title != "Brave Search" {
		t.Fatalf("choices = %v; want every engine keyed and named", definition.Choices)
	}
	if definition.CheckValue([]byte(`"altavista"`)) == nil {
		t.Fatal("the engine setting accepted an unknown engine")
	}
}
