package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/palette/aichat"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Scores of the root's rows. A higher score sorts first; rows of equal score keep the order of their sources.
// The values are the bands the prototype used, so the root's order does not change.
const (
	scoreCalculator        = 1000
	scoreTimer             = 990
	scoreQuicklink         = 980
	scoreSnippetExact      = 975
	scoreSnippet           = 970
	scoreExtension         = 960
	scoreFileFirst         = 75
	scoreClipboard         = 55
	scoreConversation      = 30
	scoreWebURL            = 150
	scoreAutomate          = 40
	scoreWebSearch         = 10
	scoreCommandSuggestion = 100
	scoreAppSuggestion     = 60
	scoreCommandAlias      = 45
)

// keyWebSearch is the usage key of the web search row. Commands and applications key themselves by title and id.
const keyWebSearch = "web:search"

// Each row function is one source of the root: it returns the rows for q, or the root rows for "". They run
// on the search goroutines over a captured rootInput; they touch the UI thread's state only inside actions,
// which run there.

// timerRows offers to start a countdown for "timer 5m" and similar queries.
func timerRows(p *App, _ *rootInput, q string) []search.Item {
	d, ok := parseTimer(q)
	if !ok || d <= 0 {
		return nil
	}
	return []search.Item{{Kind: search.KindTimer, Title: "Start timer: " + d.String(), Subtitle: "Shows a countdown in the tray", Icon: "timer", Tone: "orange", Score: scoreTimer, Actions: []search.Action{
		{Title: "Start", Run: func(h search.Host) { p.startTimer(d); h.Hide() }},
	}}}
}

// quicklinkRows answers "keyword search terms" with a search on the quicklink's site.
func quicklinkRows(_ *App, in *rootInput, q string) []search.Item {
	kw, rest, ok := strings.Cut(q, " ")
	if !ok || strings.TrimSpace(rest) == "" {
		return nil
	}
	var out []search.Item
	for _, ql := range append(slices.Clone(in.quicklinks), builtinLinks...) {
		if strings.EqualFold(kw, ql.Keyword) {
			out = append(out, search.Item{Kind: search.KindQuicklinks, Title: fmt.Sprintf("Search %s for “%s”", ql.Name, strings.TrimSpace(rest)), Subtitle: "Quicklink", Icon: "link", Tone: "blue", Score: scoreQuicklink,
				Actions: []search.Action{{Title: "Open", Run: openURL(ql.URL, strings.TrimSpace(rest))}}})
		}
	}
	return out
}

// snippetRows matches snippets by keyword, exact first and then prefix; a snippet with no keyword is found by the
// snippets view.
func snippetRows(p *App, _ *rootInput, q string) []search.Item {
	return snippetRootRows(p, q)
}

// commandRows lists the commands: the visible ones at the root, and the matches by title or keyword otherwise.
func commandRows(p *App, in *rootInput, q string) []search.Item {
	lq := strings.ToLower(q)
	var out []search.Item
	for _, cmd := range p.commandSlots(in.timerOn) {
		if isBuiltInSlot(cmd) {
			out = append(out, p.builtInRootRows(cmd.builtIn, q, in)...)
			continue
		}
		cmd := cmd
		it := search.Item{Kind: search.KindCommands, Title: cmd.Title, Subtitle: cmd.Subtitle, Icon: cmd.Icon, Tone: cmd.Tone, LearnKey: "cmd:" + cmd.Title, Actions: []search.Action{{Title: cmd.Title, Run: p.trackedRun(cmd), Dismiss: cmd.Dismiss}}}
		if commandShortcutID(cmd) != "" {
			it = p.withShortcutActions(it, commandShortcutID(cmd), in.shortcuts)
		}
		if lq == "" {
			if !cmd.Hidden {
				it.Score = scoreCommandSuggestion - cmd.Order
				out = append(out, it)
			}
			continue
		}
		s := search.Score(cmd.Title, lq)
		if cmd.PhraseMatch && s < search.PhraseScoreFloor {
			s = 0
		}
		if s == 0 && len(lq) >= 2 && strings.Contains(strings.ToLower(cmd.Keywords), lq) {
			s = scoreCommandAlias
		}
		if s == 0 {
			s = aliasMatchScore(in.aliases[commandShortcutID(cmd)], lq)
		}
		if s > 0 {
			it.Score = s
			out = append(out, it)
		}
	}
	return out
}

// clipboardTextKinds are the history kinds the root answers with: copies that are text, with or without markup.
var clipboardTextKinds = []clipboard.Kind{clipboard.KindText, clipboard.KindHTML, clipboard.KindRTF}

// clipboardRows shows the two newest text copies that contain a query of three characters or more. The history is read on the search goroutine; it locks its own state.
func clipboardRows(p *App, in *rootInput, q string) []search.Item {
	if len(strings.ToLower(q)) < 3 {
		return nil
	}
	entries := in.clip.history.Find(clipboard.Query{Text: q, Kinds: clipboardTextKinds, Limit: 2})
	historyLen := in.clip.history.Len()
	out := make([]search.Item, 0, len(entries))
	for i, e := range entries {
		ci := p.clipItem(e, historyLen)
		ci.Kind = search.KindClipboard
		ci.Score = scoreClipboard - i
		out = append(out, ci)
	}
	return out
}

// conversationRows shows up to four recent AI conversations, filtered by title when a query is typed.
func conversationRows(p *App, in *rootInput, q string) []search.Item {
	lq := strings.ToLower(q)
	var out []search.Item
	for _, ch := range in.chats {
		if ch.builder || ch.title == "" || len(out) >= 4 {
			continue
		}
		s := scoreConversation
		if lq != "" {
			if s = search.Score(ch.title, lq); s == 0 {
				continue
			}
		}
		id := ch.id
		out = append(out, search.Item{Kind: search.KindConversations, Title: "Continue conversation: " + oneLine(ch.title), Subtitle: "AI conversation", Icon: "message-circle", Tone: "yellow", TintTitle: true, Score: s,
			Actions: []search.Action{{Title: "Open", Run: func(h search.Host) { h.Push(aichat.ChatView(p.chatDeps(), id)) }}}})
	}
	return out
}

// webRows turns a query into a website when it names one, and otherwise offers the two fallbacks: Automate
// and a web search. Calculations take the place of the fallbacks.
func webRows(p *App, in *rootInput, q string) []search.Item {
	if q == "" {
		return nil
	}
	if u := search.URLFromQuery(q); u != "" {
		return []search.Item{{Kind: search.KindWeb, Title: "Open " + strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://"), Subtitle: "Open website", Icon: "globe", Tone: "blue", Score: scoreWebURL,
			Actions: []search.Action{{Title: "Open Website", Run: func(h search.Host) { h.Hide(); _ = mygo.Shell.OpenExternal(u) }}}}}
	}
	if _, ok := p.calc.Query(q); ok {
		return nil
	}
	return []search.Item{
		{Kind: search.KindSearch, Title: `Automate "` + oneLine(q) + `"`, Subtitle: "Build a reusable command with AI", Icon: "zap", Tone: "yellow", Prominent: true, Score: scoreAutomate,
			Actions: []search.Action{{Title: "Automate", Run: func(h search.Host) { h.Push(p.builderViewWith(q)) }}}},
		{Kind: search.KindSearch, Title: `Search the web for "` + oneLine(q) + `"`, Subtitle: "Search instead", Icon: "search", Tone: "blue", Score: scoreWebSearch, LearnKey: keyWebSearch,
			Actions: []search.Action{{Title: "Search the Web", Run: openURL(in.engine.URL, q)}}},
	}
}
