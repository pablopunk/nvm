package snippets

import (
	"fmt"
	"regexp"
	"slices"
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

// refusal is a save or change the user can act on. Its text is the sentence the palette shows.
type refusal string

func (r refusal) Error() string { return string(r) }

// Refusals a snippet save or change returns.
const (
	errNameRequired    = refusal("Name is required.")
	errTextRequired    = refusal("Content is required.")
	errKeywordHasSpace = refusal("A keyword is one word, with no spaces.")
	errTooLong         = refusal("That is too long for a snippet.")
	errNotFound        = refusal("This snippet no longer exists")
)

// keywordTakenError names the snippet that holds the keyword, so the user can change one of the two.
func keywordTakenError(keyword, name string) error {
	return refusal(fmt.Sprintf("Keyword “%s” is already used by “%s”. Choose another keyword.", keyword, name))
}

// Snippet is one piece of text. Keyword is stored normalized (trimmed and lower case) and is empty when the snippet is
// found by name only. The JSON shape is the one the snippets document held before this built-in.
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

// isWellFormed reports whether a snippet has the id shape snippets are given and some text to paste.
func isWellFormed(sn Snippet) bool {
	return validIDPattern.MatchString(sn.ID) && strings.TrimSpace(sn.Text) != ""
}

// usableSnippets returns the well-formed snippets in stored order.
func usableSnippets(all []Snippet) []Snippet {
	return slices.DeleteFunc(slices.Clone(all), func(sn Snippet) bool { return !isWellFormed(sn) })
}

// newID returns a snippet id that sorts by creation time and is unique within one extension's storage.
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
		return Input{}, errNameRequired
	case strings.TrimSpace(out.Text) == "":
		return Input{}, errTextRequired
	case utf8.RuneCountInString(out.Name) > maxNameRunes, utf8.RuneCountInString(out.Keyword) > maxKeywordRunes, len(out.Text) > maxTextBytes:
		return Input{}, errTooLong
	case strings.IndexFunc(out.Keyword, unicode.IsSpace) >= 0:
		return Input{}, errKeywordHasSpace
	}
	return out, nil
}
