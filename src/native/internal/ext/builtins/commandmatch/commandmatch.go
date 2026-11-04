// Package commandmatch is the rule a built-in command is found by: its title, or else its keywords when only they
// contain the query. The root's commands have always been matched this way.
//
// Layer: ext. The package imports search and the standard library.
package commandmatch

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/search"
)

// KeywordScore ranks a command found by a keyword rather than its title, below any title match.
const KeywordScore = 45

// MinKeywordQuery is the shortest query that matches a command's keywords.
const MinKeywordQuery = 2

// Score ranks a command for the lowercased query lq: its title's score, or KeywordScore when only the keywords contain
// it. Zero means no match.
func Score(title, keywords, lq string) int {
	if score := search.Score(title, lq); score > 0 {
		return score
	}
	if len(lq) >= MinKeywordQuery && strings.Contains(strings.ToLower(keywords), lq) {
		return KeywordScore
	}
	return 0
}
