package snippets

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Limits on what a snippet holds. A keyword is one word typed at the root, so it cannot contain a space.
const (
	maxNameRunes    = 120
	maxKeywordRunes = 32
	maxTextBytes    = 64 * 1024
)

// Errors of the snippet store. Each one is a message the form shows under its fields.
var (
	// ErrNotFound is returned for an id the store does not hold.
	ErrNotFound = errors.New("snippets: no such snippet")
	// ErrNameRequired is returned when a snippet has no name.
	ErrNameRequired = errors.New("snippet name is required")
	// ErrTextRequired is returned when a snippet has no text to paste.
	ErrTextRequired = errors.New("snippet text is required")
	// ErrKeywordHasSpace is returned for a keyword with a space, which could never be typed at the root as one word.
	ErrKeywordHasSpace = errors.New("keyword must be one word, with no spaces")
	// ErrTooLong is returned when the name, keyword or text is over its limit.
	ErrTooLong = errors.New("snippet is too long")
)

// KeywordTakenError is returned when another snippet already has the keyword.
type KeywordTakenError struct {
	Keyword string
	Name    string
}

// Error names the snippet that holds the keyword, so the user can change one of the two.
func (e *KeywordTakenError) Error() string {
	return fmt.Sprintf("keyword “%s” is already used by “%s”", e.Keyword, e.Name)
}

// Snippet is one piece of text. Keyword is stored normalized (trimmed and lower case) and is empty when the
// snippet is found by name only.
type Snippet struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Keyword   string    `json:"keyword,omitempty"`
	Text      string    `json:"text"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Input is what the form submits: the fields as typed.
type Input struct {
	Name    string
	Keyword string
	Text    string
}

var validIDPattern = regexp.MustCompile(`^snip-[a-z0-9-]+$`)

// validID reports whether id has the shape snippets are given.
func validID(id string) bool {
	return validIDPattern.MatchString(id)
}

// newID returns a snippet id that sorts by creation time and is unique within one store.
func newID(now time.Time, sequence uint64) string {
	return "snip-" + strconv.FormatInt(now.UnixNano(), 36) + "-" + strconv.FormatUint(sequence, 36)
}

// normalizeInput trims the name and keyword, lower-cases the keyword, and checks every field. The text is kept as
// typed, except for surrounding blank lines, which a paste never wants.
func normalizeInput(in Input) (Input, error) {
	out := Input{
		Name:    strings.TrimSpace(in.Name),
		Keyword: strings.ToLower(strings.TrimSpace(in.Keyword)),
		Text:    strings.Trim(in.Text, "\r\n"),
	}
	switch {
	case out.Name == "":
		return Input{}, ErrNameRequired
	case strings.TrimSpace(out.Text) == "":
		return Input{}, ErrTextRequired
	case utf8.RuneCountInString(out.Name) > maxNameRunes, utf8.RuneCountInString(out.Keyword) > maxKeywordRunes, len(out.Text) > maxTextBytes:
		return Input{}, ErrTooLong
	case strings.IndexFunc(out.Keyword, unicode.IsSpace) >= 0:
		return Input{}, ErrKeywordHasSpace
	}
	return out, nil
}
