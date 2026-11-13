package diag

import "regexp"

// allowedAttributes are the only keys a record may carry. Each names a closed fact about a failure, such
// as its attempt number or its operation, and never free text.
var allowedAttributes = map[string]bool{
	"attempt":     true,
	"disabled":    true,
	"duration_ms": true,
	"operation":   true,
	"outcome":     true,
	"panic":       true,
	"process":     true,
	"release":     true,
	"stage":       true,
	"status":      true,
}

// attributeValue admits identifier-like values only: lowercase words, digits and the separators used in
// versions. Spaces, slashes, quotes and long strings cannot carry a path or a sentence, so they are dropped.
var attributeValue = regexp.MustCompile(`^[a-z0-9][a-z0-9._+-]{0,63}$`)

// componentName admits the code-defined labels components are recorded under.
var componentName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// sanitizeAttributes keeps the allow-listed attributes with valid values and reports how many were
// dropped, so a caller that sends content by mistake shows up in the count rather than in the record.
func sanitizeAttributes(in map[string]string) (map[string]string, int) {
	out := make(map[string]string, len(in))
	dropped := 0
	for key, value := range in {
		if !allowedAttributes[key] || !attributeValue.MatchString(value) {
			dropped++
			continue
		}
		out[key] = value
	}
	return out, dropped
}

// sanitizeComponent returns the component label unchanged when it is a valid code label, and "invalid"
// otherwise, so no arbitrary string can become a component.
func sanitizeComponent(component string) string {
	if componentName.MatchString(component) {
		return component
	}
	return "invalid"
}
