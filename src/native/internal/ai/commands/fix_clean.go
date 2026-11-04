package commands

import (
	"regexp"
	"strings"
	"unicode"
)

const (
	textOpenTag  = "<text>"
	textCloseTag = "</text>"
	codeFence    = "```"
)

// CleanReply turns a model reply into the corrected form of input. It takes the text inside the prompt's tags, drops
// a code fence, a preamble line and wrapping quotes that input did not have, and gives back input's own leading and
// trailing whitespace and its line endings. Every step is deterministic, so the guard judges the text itself.
func CleanReply(input, reply string) string {
	core := taggedText(reply)
	core = unwrapCodeFence(core, input)
	core = dropPreamble(core, input)
	core = unwrapQuotes(core, input)
	core = matchLineEndings(strings.TrimSpace(core), input)
	return withEdgesOf(input, core)
}

// taggedText returns the text between the last close tag and the open tag before it, or the reply without stray tags
// when the reply has no tag pair.
func taggedText(reply string) string {
	start := strings.Index(reply, textOpenTag)
	end := strings.LastIndex(reply, textCloseTag)
	if start >= 0 && end > start {
		return reply[start+len(textOpenTag) : end]
	}
	return strings.NewReplacer(textOpenTag, "", textCloseTag, "").Replace(reply)
}

// unwrapCodeFence removes a fence the reply wraps around all of its text, with the optional language word on the
// fence line. Input that is itself fenced keeps its fence.
func unwrapCodeFence(core, input string) string {
	trimmed := strings.TrimSpace(core)
	if !isFenced(trimmed) || strings.HasPrefix(strings.TrimSpace(input), codeFence) {
		return core
	}
	body := trimmed[len(codeFence) : len(trimmed)-len(codeFence)]
	firstLine, rest, multiline := strings.Cut(body, "\n")
	if multiline && !strings.ContainsAny(strings.TrimSpace(firstLine), " \t") {
		return rest
	}
	return body
}

// isFenced reports whether text starts and ends with a code fence.
func isFenced(text string) bool {
	return len(text) >= 2*len(codeFence) && strings.HasPrefix(text, codeFence) && strings.HasSuffix(text, codeFence)
}

// preamblePattern matches the opening words of a line a model adds before the text, such as "Here is the corrected
// text:". politePattern matches a line that is only an acknowledgement, such as "Sure!"; a line with words after it is
// text and is kept.
var (
	preamblePattern = regexp.MustCompile(`^(here('s| is| are)|the corrected|corrected|revised|fixed|proofread)\b`)
	politePattern   = regexp.MustCompile(`^(sure|certainly|of course)[,!.]\s*$`)
)

// dropPreamble removes the first line of the reply when it is a preamble and input's first line is not one. The line
// must be followed by the text, so a reply that is only a preamble stays as it is and is refused by the guard.
func dropPreamble(core, input string) string {
	first, rest, hasMore := strings.Cut(core, "\n")
	if !hasMore || !isPreambleLine(first) || isPreambleLine(firstLineOf(input)) {
		return core
	}
	return strings.TrimLeft(rest, "\r\n \t")
}

// isPreambleLine reports whether line is a model's preamble: a line that opens like a correction notice and ends with
// a colon, or an acknowledgement followed by punctuation.
func isPreambleLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	if preamblePattern.MatchString(lower) && strings.HasSuffix(trimmed, ":") {
		return true
	}
	return politePattern.MatchString(lower)
}

// firstLineOf returns text up to its first line break.
func firstLineOf(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

// quotePairs are the wrapping quotes a model may add around the whole reply.
var quotePairs = [][2]string{{`"`, `"`}, {"'", "'"}, {"“", "”"}, {"‘", "’"}, {"«", "»"}, {"„", "“"}}

// unwrapQuotes removes one pair of quotes that wrap all of the reply, unless input starts with the same quote.
func unwrapQuotes(core, input string) string {
	trimmed := strings.TrimSpace(core)
	inputTrimmed := strings.TrimSpace(input)
	for _, pair := range quotePairs {
		open, closing := pair[0], pair[1]
		if len(trimmed) < len(open)+len(closing) || !strings.HasPrefix(trimmed, open) || !strings.HasSuffix(trimmed, closing) {
			continue
		}
		if strings.HasPrefix(inputTrimmed, open) {
			return core
		}
		return trimmed[len(open) : len(trimmed)-len(closing)]
	}
	return core
}

// matchLineEndings writes the reply's line breaks as CRLF when input uses CRLF and the reply has none.
func matchLineEndings(core, input string) string {
	if strings.Contains(input, "\r\n") && !strings.Contains(core, "\r") {
		return strings.ReplaceAll(core, "\n", "\r\n")
	}
	return core
}

// withEdgesOf puts input's leading and trailing whitespace around core.
func withEdgesOf(input, core string) string {
	lead := input[:len(input)-len(strings.TrimLeftFunc(input, unicode.IsSpace))]
	rest := strings.TrimLeftFunc(input, unicode.IsSpace)
	trail := rest[len(strings.TrimRightFunc(rest, unicode.IsSpace)):]
	return lead + core + trail
}
