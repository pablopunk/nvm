package logx

import (
	"regexp"
	"strings"
	"unicode"
)

// Each rule is applied in order to free text. Patterns are anchored on words that carry secrets, so
// ordinary prose survives. Replacements are literal: RedactedValue contains no "$".
var (
	headerPattern   = regexp.MustCompile(`(?i)\b(authorization|set-cookie|cookie)(["']?\s*[:=]\s*)[^\r\n]*`)
	bearerPattern   = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]+`)
	providerPattern = regexp.MustCompile(`\b(?:sk|pk|rk|xox[abprs]|gh[pousr]|github_pat)[-_][A-Za-z0-9_-]{12,}|\bAKIA[0-9A-Z]{16}\b`)
	urlPattern      = regexp.MustCompile(`(?i)\b([a-z][a-z0-9+.-]*://[^\s"'<>?#]*)([?#][^\s"'<>]*)`)
	keyValuePattern = regexp.MustCompile(`(?i)([a-z0-9_.-]*(?:token|secret|password|passwd|authorization|cookie|key)[a-z0-9_.-]*["']?\s*[=:]\s*)(\[REDACTED\]|"[^"]*"|'[^']*'|[^\s&;,"'\]}]+)`)
	base64Pattern   = regexp.MustCompile(`[A-Za-z0-9+/_-]{40,}={0,2}`)
	hexPattern      = regexp.MustCompile(`\b[0-9a-fA-F]{32,}\b`)
)

// Text returns s with every secret-looking part replaced by RedactedValue.
func (r *Redactor) Text(s string) string {
	s = headerPattern.ReplaceAllString(s, "${1}${2}"+RedactedValue)
	s = bearerPattern.ReplaceAllString(s, "Bearer "+RedactedValue)
	s = providerPattern.ReplaceAllString(s, RedactedValue)
	s = urlPattern.ReplaceAllStringFunc(s, redactURLValues)
	s = keyValuePattern.ReplaceAllString(s, "${1}"+RedactedValue)
	s = base64Pattern.ReplaceAllStringFunc(s, redactMixedRun)
	s = hexPattern.ReplaceAllStringFunc(s, redactHexRun)
	return s
}

// redactURLValues keeps the scheme, host, path and every query or fragment name, and replaces each
// value. Values are removed whatever their name, because OAuth callbacks carry secrets under names such
// as code or state.
func redactURLValues(match string) string {
	cut := strings.IndexAny(match, "?#")
	if cut < 0 {
		return match
	}
	head, separator, tail := match[:cut], match[cut:cut+1], match[cut+1:]
	var pairs []string
	for _, pair := range strings.Split(tail, "&") {
		if pair == "" {
			continue
		}
		if eq := strings.IndexByte(pair, '='); eq >= 0 {
			pairs = append(pairs, pair[:eq+1]+RedactedValue)
		} else {
			pairs = append(pairs, RedactedValue)
		}
	}
	return head + separator + strings.Join(pairs, "&")
}

// redactMixedRun removes a long run that has digits, upper and lower case letters: the shape of random
// base64 keys. Plain paths and lowercase identifiers have no digits or no mixed case and survive.
func redactMixedRun(run string) string {
	var digits, upper, lower bool
	for _, r := range run {
		switch {
		case unicode.IsDigit(r):
			digits = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		}
	}
	if digits && upper && lower {
		return RedactedValue
	}
	return run
}

// redactHexRun removes a long hexadecimal run that mixes digits and letters, such as a hex key or an
// unseparated UUID. An all-digit run (a timestamp, a count) survives.
func redactHexRun(run string) string {
	hasDigit := strings.ContainsAny(run, "0123456789")
	hasLetter := strings.ContainsAny(run, "abcdefABCDEF")
	if hasDigit && hasLetter {
		return RedactedValue
	}
	return run
}
