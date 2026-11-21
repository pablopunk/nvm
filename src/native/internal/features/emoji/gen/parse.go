package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// literalParser reads the JavaScript literal subset the TypeScript catalog uses: arrays, quoted
// strings with common escapes, and non-negative integers. Comments are skipped. Anything else is
// an error, so a change to the source that the generator cannot read fails loudly.
type literalParser struct {
	src string
	pos int
}

const fromEntriesPrefix = "Object.fromEntries("

// parseExportedArray returns the array assigned to `export const <name>`. A value wrapped in
// Object.fromEntries(...) is unwrapped, since the pairs it holds are the array we need.
func parseExportedArray(src, name string) ([]any, error) {
	start := strings.Index(src, "export const "+name)
	if start < 0 {
		return nil, fmt.Errorf("declaration %s not found", name)
	}
	equals := strings.IndexByte(src[start:], '=')
	if equals < 0 {
		return nil, fmt.Errorf("declaration %s has no initializer", name)
	}
	p := &literalParser{src: src, pos: start + equals + 1}
	p.skipSpace()
	wrapped := strings.HasPrefix(p.src[p.pos:], fromEntriesPrefix)
	if wrapped {
		p.pos += len(fromEntriesPrefix)
	}
	value, err := p.value()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if wrapped {
		if err := p.expect(')'); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s is not an array", name)
	}
	return list, nil
}

func (p *literalParser) value() (any, error) {
	p.skipSpace()
	if p.pos >= len(p.src) {
		return nil, p.errorf("unexpected end of input")
	}
	switch c := p.src[p.pos]; {
	case c == '[':
		return p.list()
	case c == '"' || c == '\'':
		return p.str()
	case c >= '0' && c <= '9':
		return p.number()
	default:
		return nil, p.errorf("unexpected character %q", c)
	}
}

func (p *literalParser) list() ([]any, error) {
	p.pos++
	items := []any{}
	for {
		p.skipSpace()
		if p.pos >= len(p.src) {
			return nil, p.errorf("unterminated array")
		}
		if p.src[p.pos] == ']' {
			p.pos++
			return items, nil
		}
		item, err := p.value()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		p.skipSpace()
		switch {
		case p.pos < len(p.src) && p.src[p.pos] == ',':
			p.pos++
		case p.pos < len(p.src) && p.src[p.pos] == ']':
		default:
			return nil, p.errorf("expected ',' or ']' in array")
		}
	}
}

func (p *literalParser) str() (string, error) {
	quote := p.src[p.pos]
	p.pos++
	var b strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == quote:
			p.pos++
			return b.String(), nil
		case c == '\\':
			if err := p.escape(&b); err != nil {
				return "", err
			}
		case c == '\n':
			return "", p.errorf("newline inside string")
		default:
			b.WriteByte(c)
			p.pos++
		}
	}
	return "", p.errorf("unterminated string")
}

func (p *literalParser) escape(b *strings.Builder) error {
	p.pos++
	if p.pos >= len(p.src) {
		return p.errorf("dangling backslash")
	}
	c := p.src[p.pos]
	p.pos++
	switch c {
	case '\\', '\'', '"':
		b.WriteByte(c)
	case 'n':
		b.WriteByte('\n')
	case 't':
		b.WriteByte('\t')
	case 'r':
		b.WriteByte('\r')
	case 'x':
		return p.hexEscape(b, 2)
	case 'u':
		if p.pos < len(p.src) && p.src[p.pos] == '{' {
			end := strings.IndexByte(p.src[p.pos:], '}')
			if end < 0 {
				return p.errorf("unterminated \\u{...} escape")
			}
			hex := p.src[p.pos+1 : p.pos+end]
			p.pos += end + 1
			return p.writeCodePoint(b, hex)
		}
		return p.hexEscape(b, 4)
	default:
		return p.errorf("unsupported escape \\%c", c)
	}
	return nil
}

func (p *literalParser) hexEscape(b *strings.Builder, digits int) error {
	if p.pos+digits > len(p.src) {
		return p.errorf("short hex escape")
	}
	hex := p.src[p.pos : p.pos+digits]
	p.pos += digits
	return p.writeCodePoint(b, hex)
}

func (p *literalParser) writeCodePoint(b *strings.Builder, hex string) error {
	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil || value > utf8.MaxRune || (value >= 0xD800 && value <= 0xDFFF) {
		return p.errorf("invalid code point escape %q", hex)
	}
	b.WriteRune(rune(value))
	return nil
}

func (p *literalParser) number() (int, error) {
	start := p.pos
	for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
		p.pos++
	}
	value, err := strconv.Atoi(p.src[start:p.pos])
	if err != nil {
		return 0, p.errorf("bad number: %v", err)
	}
	return value, nil
}

func (p *literalParser) expect(want byte) error {
	p.skipSpace()
	if p.pos >= len(p.src) || p.src[p.pos] != want {
		return p.errorf("expected %q", want)
	}
	p.pos++
	return nil
}

func (p *literalParser) skipSpace() {
	for p.pos < len(p.src) {
		rest := p.src[p.pos:]
		switch {
		case rest[0] == ' ' || rest[0] == '\t' || rest[0] == '\n' || rest[0] == '\r':
			p.pos++
		case strings.HasPrefix(rest, "//"):
			end := strings.IndexByte(rest, '\n')
			if end < 0 {
				p.pos = len(p.src)
			} else {
				p.pos += end + 1
			}
		case strings.HasPrefix(rest, "/*"):
			end := strings.Index(rest[2:], "*/")
			if end < 0 {
				p.pos = len(p.src)
			} else {
				p.pos += end + 4
			}
		default:
			return
		}
	}
}

func (p *literalParser) errorf(format string, args ...any) error {
	line := strings.Count(p.src[:min(p.pos, len(p.src))], "\n") + 1
	return fmt.Errorf("line %d: %s", line, fmt.Sprintf(format, args...))
}
