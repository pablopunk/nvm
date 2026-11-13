package calc

import (
	"errors"
	"strings"
)

// errSyntax reports any expression the grammar does not accept. Callers only need to know that it failed.
var errSyntax = errors.New("calc: expression does not parse")

type tokenKind uint8

const (
	tokenNumber tokenKind = iota
	tokenIdentifier
	tokenOperator
	tokenPercent
	tokenLeftParen
	tokenRightParen
	tokenComma
	tokenEOF
)

// token is one lexical unit. number is set for numbers, text for identifiers (lower-cased) and operators.
type token struct {
	kind   tokenKind
	number float64
	text   string
}

// tokenize splits an expression into tokens. Numbers take their magnitude suffix here when the suffix is not
// followed by a letter, so "50k" is one token and "5kb" is an error, as in the TypeScript original.
func tokenize(expression string) ([]token, error) {
	runes := []rune(expression)
	var tokens []token
	for index := 0; index < len(runes); {
		r := runes[index]
		switch {
		case isJSSpace(r):
			index++
		case r == '(':
			tokens = append(tokens, token{kind: tokenLeftParen})
			index++
		case r == ')':
			tokens = append(tokens, token{kind: tokenRightParen})
			index++
		case r == ',':
			tokens = append(tokens, token{kind: tokenComma})
			index++
		case r == '%':
			tokens = append(tokens, token{kind: tokenPercent})
			index++
		case isOperatorRune(r):
			tokens = append(tokens, token{kind: tokenOperator, text: string(r)})
			index++
		case isNumberStart(runes, index):
			value, next, err := readNumber(runes, index)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token{kind: tokenNumber, number: value})
			index = next
		case isLetterRune(r):
			start := index
			for index < len(runes) && isLetterRune(runes[index]) {
				index++
			}
			tokens = append(tokens, token{kind: tokenIdentifier, text: strings.ToLower(string(runes[start:index]))})
		default:
			return nil, errSyntax
		}
	}
	return append(tokens, token{kind: tokenEOF}), nil
}

// isOperatorRune reports whether r is one of the binary operators.
func isOperatorRune(r rune) bool {
	return r == '+' || r == '-' || r == '*' || r == '/' || r == '^'
}

// isNumberStart reports whether a number begins at index: a digit, or a dot followed by a digit.
func isNumberStart(runes []rune, index int) bool {
	if isDigit(runes[index]) {
		return true
	}
	return runes[index] == '.' && index+1 < len(runes) && isDigit(runes[index+1])
}

// readNumber reads digits, one decimal point and thousands groups, then an optional magnitude suffix. It returns
// the value and the index after the number.
func readNumber(runes []rune, start int) (float64, int, error) {
	index := start
	sawDot := false
	for index < len(runes) {
		r := runes[index]
		if isDigit(r) {
			index++
			continue
		}
		if r == '.' && !sawDot {
			sawDot = true
			index++
			continue
		}
		if r == ',' && startsThousandsGroup(runes, index) {
			index++
			continue
		}
		break
	}
	value, ok := parseNumberLiteral(string(runes[start:index]))
	if !ok {
		return 0, 0, errSyntax
	}
	followedByLetter := index+1 < len(runes) && isASCIILetterRune(runes[index+1])
	if index < len(runes) && !followedByLetter {
		if multiplier, ok := suffixMultiplier(strings.ToLower(string(runes[index]))); ok {
			value *= multiplier
			index++
		}
	}
	return value, index, nil
}

// startsThousandsGroup reports whether the comma at index is followed by exactly three digits and then a
// non-digit, as /^,\d{3}(?!\d)/ requires.
func startsThousandsGroup(runes []rune, index int) bool {
	if index+3 >= len(runes) || !isDigit(runes[index+1]) || !isDigit(runes[index+2]) || !isDigit(runes[index+3]) {
		return false
	}
	return index+4 >= len(runes) || !isDigit(runes[index+4])
}
