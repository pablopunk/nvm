package files

import (
	"testing"
)

// scoreOf scores an already-lowercased name against an already-lowercased query, computing the query words the way
// a search does.
func scoreOf(name, query string) int {
	var words [][]byte
	if w := splitWords([]byte(query)); len(w) >= 2 {
		words = w
	}
	return scoreFuzzy([]byte(name), []byte(query), words)
}

// TestScoreBandsOfTheRankingContract checks one example per band of the palette ranking contract.
func TestScoreBandsOfTheRankingContract(t *testing.T) {
	cases := []struct {
		band, name, query string
		want              int
	}{
		{"exact", "readme", "readme", scoreExact},
		{"phrase: whole word inside a name", "the read notes", "read", scorePhrase},
		{"phrase: punctuation is a boundary", "readme.md", "readme", scorePhrase},
		{"starts with", "readmefile", "readme", scoreStartsWith},
		{"substring inside a word", "bread", "read", scoreContains},
		{"ordered word prefixes from the first word", "main readme", "ma re", scoreWordPrefixAt0},
		{"ordered word prefixes later in the name", "the main readme", "ma re", scoreWordPrefix},
		{"unordered word prefixes", "readme main", "ma re", scoreWordUnordered},
		{"subsequence", "main readme", "mnr", scoreSubsequence},
		{"character set in another order", "readme", "emr", scoreCharset},
		{"no match", "readme", "zz", 0},
		{"repeated character needs enough copies", "readme", "eee", 0},
		{"unicode words are split and matched", "café au lait", "café", scorePhrase},
	}
	for _, c := range cases {
		t.Run(c.band, func(t *testing.T) {
			if got := scoreOf(c.name, c.query); got != c.want {
				t.Fatalf("score(%q, %q) = %d, want %d", c.name, c.query, got, c.want)
			}
		})
	}
}

// TestScoreBandsDescendFromExactToCharacterSet checks the ordering the bands promise: a stronger match never scores
// below a weaker one.
func TestScoreBandsDescendFromExactToCharacterSet(t *testing.T) {
	order := []int{scoreExact, scorePhrase, scoreStartsWith, scoreContains, scoreSubsequence, scoreCharset}
	for i := 1; i < len(order); i++ {
		if order[i-1] <= order[i] {
			t.Fatalf("band %d must score above band %d", order[i-1], order[i])
		}
	}
	if !(scoreWordPrefixAt0 > scoreWordPrefix && scoreWordPrefix > scoreWordUnordered && scoreWordUnordered > scoreContains) {
		t.Fatal("word-prefix bands must sit between the phrase and substring bands")
	}
}

// TestSplitWordsSeparatesOnNonLetters checks the word split that the word-prefix bands use.
func TestSplitWordsSeparatesOnNonLetters(t *testing.T) {
	words := splitWords([]byte("  foo-bar_2 baz.qux "))
	want := []string{"foo", "bar", "2", "baz", "qux"}
	if len(words) != len(want) {
		t.Fatalf("splitWords = %q, want %q", words, want)
	}
	for i := range want {
		if string(words[i]) != want[i] {
			t.Fatalf("splitWords[%d] = %q, want %q", i, words[i], want[i])
		}
	}
}

// TestCompareNamesFoldIsCaseInsensitiveThenByLength checks the name order used by name sorts and for ties.
func TestCompareNamesFoldIsCaseInsensitiveThenByLength(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"apple", "Banana", -1},
		{"Banana", "apple", 1},
		{"same", "SAME", 0},
		{"ab", "abc", -1},
		{"abc", "ab", 1},
	}
	for _, c := range cases {
		if got := compareNamesFold([]byte(c.a), []byte(c.b)); got != c.want {
			t.Errorf("compareNamesFold(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestTopKKeepsTheBestHitsInOrder checks that the selection keeps exactly the best limit hits, best first, whatever the
// arrival order.
func TestTopKKeepsTheBestHitsInOrder(t *testing.T) {
	better := func(a, b hit) bool { return a.score > b.score || (a.score == b.score && a.idx < b.idx) }
	top := newTopK(3)
	arrivals := []hit{{idx: 1, score: 20}, {idx: 2, score: 90}, {idx: 3, score: 50}, {idx: 4, score: 90}, {idx: 5, score: 10}, {idx: 6, score: 80}}
	for _, h := range arrivals {
		top.offer(h, better)
	}
	got := top.sorted(better)
	want := []hit{{idx: 2, score: 90}, {idx: 4, score: 90}, {idx: 6, score: 80}}
	if len(got) != len(want) {
		t.Fatalf("kept %d hits, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("hit %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
