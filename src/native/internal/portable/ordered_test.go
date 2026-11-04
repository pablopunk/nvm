package portable

import (
	"encoding/json"
	"testing"
)

func TestDocumentRoundTripKeepsNumbersMarkupAndOrder(t *testing.T) {
	text := `{
  "version": 1,
  "zeta": 1.50,
  "settings": {},
  "alpha": [
    "<&>",
    null,
    true,
    {},
    []
  ],
  "shortcuts": {}
}
`
	obj, err := decodeDocument([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	out, err := encodeDocument(obj)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != text {
		t.Errorf("round trip changed the document:\n%s\nwant:\n%s", out, text)
	}
}

func TestEncodedDocumentIsValidJSON(t *testing.T) {
	doc := newDocument()
	doc.SetValue("Alt+Space", "settings", "paletteHotkey")
	doc.SetValue(nil, "shortcuts", "command:Clipboard History")
	out, err := doc.encode()
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("encoded document is not JSON: %v", err)
	}
}

func TestRemoveKeepsTheOtherKeysInPlace(t *testing.T) {
	doc := newDocument()
	doc.SetValue("a", "shortcuts", "first")
	doc.SetValue("b", "shortcuts", "second")
	doc.Remove("shortcuts", "first")
	if keys := doc.Keys("shortcuts"); len(keys) != 1 || keys[0] != "second" {
		t.Errorf("keys after remove = %v", keys)
	}
}
