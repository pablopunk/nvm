package snippets

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

type fixture struct {
	t     *testing.T
	set   *fake.Set
	st    *store.Store
	items *Store
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	set := fake.NewSet()
	f := &fixture{t: t, set: set}
	f.restart(false)
	t.Cleanup(func() { _ = f.st.Close() })
	return f
}

// restart opens the store again over the same files, as a new process would.
func (f *fixture) restart(readOnly bool) {
	f.t.Helper()
	st, err := store.Open(f.set.FS, f.set.Clock, "/snippets")
	if err != nil {
		f.t.Fatal(err)
	}
	f.st = st
	f.items = New(st, f.set.Clock, readOnly)
}

func (f *fixture) create(name, keyword, text string) Snippet {
	f.t.Helper()
	sn, err := f.items.Create(Input{Name: name, Keyword: keyword, Text: text})
	if err != nil {
		f.t.Fatalf("create %q: %v", name, err)
	}
	return sn
}

func TestCreateValidatesEveryFieldAndNormalizesTheKeyword(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name string
		in   Input
		want error
	}{
		{"name required", Input{Name: "  ", Text: "x"}, ErrNameRequired},
		{"text required", Input{Name: "n", Text: "  \n "}, ErrTextRequired},
		{"keyword is one word", Input{Name: "n", Keyword: "two words", Text: "x"}, ErrKeywordHasSpace},
		{"keyword limit", Input{Name: "n", Keyword: strings.Repeat("k", maxKeywordRunes+1), Text: "x"}, ErrTooLong},
		{"name limit", Input{Name: strings.Repeat("n", maxNameRunes+1), Text: "x"}, ErrTooLong},
		{"text limit", Input{Name: "n", Text: strings.Repeat("t", maxTextBytes+1)}, ErrTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.items.Create(tc.in); !errors.Is(err, tc.want) {
				t.Fatalf("Create = %v, want %v", err, tc.want)
			}
		})
	}
	sn := f.create("  Email signature ", "  ;SIG  ", "\nBest,\nAna\n")
	if sn.Name != "Email signature" || sn.Keyword != ";sig" || sn.Text != "Best,\nAna" {
		t.Fatalf("normalized snippet = %+v", sn)
	}
	if !validID(sn.ID) {
		t.Fatalf("id %q has the wrong shape", sn.ID)
	}
}

func TestKeywordIsUniqueAndAnUpdateKeepsItsOwn(t *testing.T) {
	f := newFixture(t)
	first := f.create("Sig", ";sig", "a")
	second := f.create("Address", "", "b")

	_, err := f.items.Create(Input{Name: "Other", Keyword: ";SIG", Text: "c"})
	var taken *KeywordTakenError
	if !errors.As(err, &taken) || taken.Name != "Sig" {
		t.Fatalf("duplicate keyword = %v, want KeywordTakenError naming Sig", err)
	}
	if _, err := f.items.Update(second.ID, Input{Name: "Address", Keyword: ";sig", Text: "b"}); !errors.As(err, &taken) {
		t.Fatalf("update onto a taken keyword = %v", err)
	}
	if _, err := f.items.Update(first.ID, Input{Name: "Sig v2", Keyword: ";sig", Text: "a2"}); err != nil {
		t.Fatalf("update keeping its own keyword = %v", err)
	}
	if _, err := f.items.Create(Input{Name: "No keyword", Text: "d"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.items.Create(Input{Name: "Also no keyword", Text: "e"}); err != nil {
		t.Fatalf("two snippets without a keyword = %v", err)
	}
}

func TestMatchFindsExactKeywordsBeforePrefixes(t *testing.T) {
	f := newFixture(t)
	f.create("Sig", ";sig", "a")
	f.create("Signature block", ";sigblock", "b")
	f.create("Name only", "", "c")

	exact, prefix := f.items.Match(";SIG")
	if len(exact) != 1 || exact[0].Name != "Sig" {
		t.Fatalf("exact = %+v", exact)
	}
	if len(prefix) != 1 || prefix[0].Name != "Signature block" {
		t.Fatalf("prefix = %+v", prefix)
	}
	exact, prefix = f.items.Match("   ")
	if exact != nil || prefix != nil {
		t.Fatal("a blank query matched something")
	}
}

func TestUpdateAndDeleteOnMissingSnippetsFail(t *testing.T) {
	f := newFixture(t)
	if _, err := f.items.Update("snip-missing", Input{Name: "n", Text: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update missing = %v", err)
	}
	if err := f.items.Delete("snip-missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing = %v", err)
	}
	sn := f.create("Gone", ";gone", "x")
	if err := f.items.Delete(sn.ID); err != nil {
		t.Fatal(err)
	}
	if len(f.items.List()) != 0 {
		t.Fatal("deleted snippet still listed")
	}
}

func TestListIsSortedByName(t *testing.T) {
	f := newFixture(t)
	f.create("beta", "", "1")
	f.create("Alpha", "", "2")
	list := f.items.List()
	if list[0].Name != "Alpha" || list[1].Name != "beta" {
		t.Fatalf("order = %q, %q", list[0].Name, list[1].Name)
	}
}

func TestExpandFillsPlaceholdersInOnePass(t *testing.T) {
	now := time.Date(2026, time.November, 3, 9, 30, 0, 0, time.UTC)
	cases := []struct {
		name, text, clip, want string
	}{
		{"clipboard", "quote: {clipboard}", "hello", "quote: hello"},
		{"date", "on {date}", "", "on 2026-11-03"},
		{"unknown braces stay", "{cursor} and {other}", "x", "{cursor} and {other}"},
		{"clipboard with a placeholder is pasted as written", "{clipboard}", "{date}", "{date}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Expand(tc.text, tc.clip, now); got != tc.want {
				t.Fatalf("Expand = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPasteSendsTheGestureAndFallsBackToCopyWhenRefused(t *testing.T) {
	f := newFixture(t)
	sn := Snippet{ID: "snip-x", Name: "n", Text: "from {clipboard}"}
	f.set.Clipboard.WriteText("source")

	pasted, err := Paste(sn, f.set.Clock.Now(), f.set.Clipboard, f.set.Paster, "org.example.Editor")
	if err != nil || !pasted {
		t.Fatalf("paste = %v, %v", pasted, err)
	}
	if got, _ := f.set.Clipboard.ReadText(); got != "from source" {
		t.Fatalf("clipboard = %q", got)
	}
	if reqs := f.set.Paster.Requests(); len(reqs) != 1 || reqs[0] != "org.example.Editor" {
		t.Fatalf("paste requests = %v", reqs)
	}

	f.set.Paster.FailNext(fake.OpPasteIntoFrontmost, platform.ErrConflict)
	plain := Snippet{ID: "snip-y", Name: "n", Text: "plain text"}
	pasted, err = Paste(plain, f.set.Clock.Now(), f.set.Clipboard, f.set.Paster, "org.example.Editor")
	if pasted || !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("refused paste = %v, %v", pasted, err)
	}
	if got, _ := f.set.Clipboard.ReadText(); got != "plain text" {
		t.Fatalf("refused paste left clipboard %q, want the text kept for a manual paste", got)
	}
}

func TestReadOnlyStoreListsButWritesNothing(t *testing.T) {
	f := newFixture(t)
	f.create("Shared", ";shared", "text")
	if err := f.st.Flush(); err != nil {
		t.Fatal(err)
	}
	f.restart(true)
	if !f.items.ReadOnly() {
		t.Fatal("store is not read-only")
	}
	if len(f.items.List()) != 1 {
		t.Fatal("read-only store does not list the snippet")
	}
	if _, err := f.items.Create(Input{Name: "n", Text: "x"}); !errors.Is(err, store.ErrReadOnly) {
		t.Fatalf("create = %v, want ErrReadOnly", err)
	}
	if err := f.items.Delete(f.items.List()[0].ID); !errors.Is(err, store.ErrReadOnly) {
		t.Fatalf("delete = %v, want ErrReadOnly", err)
	}
}

func TestSnippetsSurviveARestart(t *testing.T) {
	f := newFixture(t)
	f.create("Persisted", ";keep", "kept text")
	if err := f.st.Flush(); err != nil {
		t.Fatal(err)
	}
	f.restart(false)
	exact, _ := f.items.Match(";keep")
	if len(exact) != 1 || exact[0].Text != "kept text" {
		t.Fatalf("after restart: %+v", exact)
	}
}
