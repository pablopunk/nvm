package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// Scores of the root's rows. A higher score sorts first; rows of equal score keep the order of their sources.
// The values are the bands the prototype used, so the root's order does not change.
const (
	scoreCalculator        = 1000
	scoreTimer             = 990
	scoreQuicklink         = 980
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

// calculatorRows shows the result of a calculation or unit conversion.
func calculatorRows(_ *App, _ *rootInput, q string) []search.Item {
	disp, cp, ok := calcResult(q)
	if !ok {
		return nil
	}
	return []search.Item{{Kind: search.KindCalculator, Title: disp, Subtitle: "Copy result", Icon: "calculator", Tone: "orange", Score: scoreCalculator, Actions: []search.Action{
		{Title: "Copy Result", Run: func(h search.Host) { mygo.Clipboard.WriteText(cp); h.Hide() }},
		{Title: "Paste Result", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(h search.Host) { h.Hide(); pasteText(cp) }},
	}}}
}

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

// snippetRows matches snippets by keyword prefix; a snippet with no keyword is found by the snippets view.
func snippetRows(p *App, in *rootInput, q string) []search.Item {
	lq := strings.ToLower(q)
	var out []search.Item
	for _, sn := range in.snippets {
		if sn.Keyword != "" && lq != "" && strings.HasPrefix(strings.ToLower(sn.Keyword), lq) {
			it := p.snippetItem(sn)
			it.Kind = search.KindSnippets
			it.Score = scoreSnippet
			out = append(out, it)
		}
	}
	return out
}

// extensionRows asks every enabled extension for its rows. Each extension's search runs under its own lock.
func extensionRows(_ *App, in *rootInput, q string) []search.Item {
	var out []search.Item
	for i := range in.exts {
		e := &in.exts[i]
		if in.disabled[e.ID] {
			continue
		}
		for _, x := range safeSearch(e, q) {
			x := x
			act := search.Action{Title: "Copy", Run: func(h search.Host) { mygo.Clipboard.WriteText(x.Value); h.Hide() }}
			if x.URL != "" {
				act = search.Action{Title: "Open", Run: func(h search.Host) { h.Hide(); _ = mygo.Shell.OpenExternal(x.URL) }}
			}
			out = append(out, search.Item{Kind: e.Title, Title: x.Title, Subtitle: x.Subtitle, Icon: "puzzle", Tone: "purple", Score: scoreExtension, Actions: []search.Action{act}})
		}
	}
	return out
}

// commandRows lists the commands: the visible ones at the root, and the matches by title or keyword otherwise.
func commandRows(p *App, in *rootInput, q string) []search.Item {
	lq := strings.ToLower(q)
	var out []search.Item
	for _, cmd := range p.commands(in.timerOn) {
		cmd := cmd
		it := search.Item{Kind: search.KindCommands, Title: cmd.Title, Subtitle: cmd.Subtitle, Icon: cmd.Icon, Tone: cmd.Tone, LearnKey: "cmd:" + cmd.Title, Actions: []search.Action{{Title: "Open", Run: cmd.Run, Dismiss: cmd.Dismiss}}}
		if lq == "" {
			if !cmd.Hidden {
				it.Score = scoreCommandSuggestion - cmd.Order
				out = append(out, it)
			}
			continue
		}
		s := search.Score(cmd.Title, lq)
		if s == 0 && len(lq) >= 2 && strings.Contains(strings.ToLower(cmd.Keywords), lq) {
			s = scoreCommandAlias
		}
		if s > 0 {
			it.Score = s
			out = append(out, it)
		}
	}
	return out
}

// fileRows shows the best four matches of the file index once two characters are typed.
func fileRows(p *App, in *rootInput, q string) []search.Item {
	if len(strings.ToLower(q)) < 2 || !in.filesDone {
		return nil
	}
	var out []search.Item
	for i, path := range searchPaths(in.files, q, 4) {
		fi := p.fileItem(path)
		fi.Kind = search.KindFiles
		fi.Score = scoreFileFirst - i
		out = append(out, fi)
	}
	return out
}

// clipboardRows shows the two newest text copies that contain a query of three characters or more.
func clipboardRows(p *App, in *rootInput, q string) []search.Item {
	lq := strings.ToLower(q)
	if len(lq) < 3 {
		return nil
	}
	var out []search.Item
	for _, e := range in.clips {
		if len(out) >= 2 {
			break
		}
		if e.Kind == "text" && strings.Contains(strings.ToLower(e.Text), lq) {
			ci := p.clipItem(e)
			ci.Kind = search.KindClipboard
			ci.Score = scoreClipboard - len(out)
			out = append(out, ci)
		}
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
		chat := ch.chat
		out = append(out, search.Item{Kind: search.KindConversations, Title: "Continue conversation: " + oneLine(ch.title), Subtitle: "AI conversation", Icon: "message-circle", Tone: "yellow", TintTitle: true, Score: s,
			Actions: []search.Action{{Title: "Open", Run: func(h search.Host) { h.Push(p.chatView(chat)) }}}})
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
	if _, _, ok := calcResult(q); ok {
		return nil
	}
	return []search.Item{
		{Kind: search.KindSearch, Title: `Automate "` + oneLine(q) + `"`, Subtitle: "Build a reusable command with AI", Icon: "zap", Tone: "yellow", Prominent: true, Score: scoreAutomate,
			Actions: []search.Action{{Title: "Automate", Run: func(h search.Host) { h.Push(p.builderViewWith(q)) }}}},
		{Kind: search.KindSearch, Title: `Search the web for "` + oneLine(q) + `"`, Subtitle: "Search instead", Icon: "search", Tone: "blue", Score: scoreWebSearch, LearnKey: keyWebSearch,
			Actions: []search.Action{{Title: "Search the Web", Run: openURL(in.engine.URL, q)}}},
	}
}
