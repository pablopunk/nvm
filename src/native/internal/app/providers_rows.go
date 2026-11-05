package app

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/palette/aichat"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Scores of the root's rows. A higher score sorts first; rows of equal score are ordered as 1.x orders them (compareRows).
// The values are the bands the prototype used, so the root's order does not change.
const (
	scoreExtension    = 960
	scoreFileFirst    = 75
	scoreClipboard    = 55
	scoreAutomate     = 40
	scoreCommandAlias = 45
)

// The empty root scores its rows as 1.x does, on 1.x's scale: a copy 60, an app 30 plus the 25 an app result gets,
// a conversation 13, a recent file the rest of the band below the commands. A command with no 1.x score sits at the
// 1.x default for commands.
const (
	scoreRootClipboard    = 60
	scoreRootApplication  = 55
	scoreRootConversation = 13
	scoreRootCommand      = 12
	rootClipboardLimit    = 10
	// rootSearchClipboardLimit is how many copies a search from the root lists, as 1.x does.
	rootSearchClipboardLimit = 5
)

// checkForUpdatesTitle names the command that checks for an update.
const checkForUpdatesTitle = "Check for Updates"

// rootCommandScores are the 1.x scores of the commands the empty root lists.
var rootCommandScores = map[string]int{"Clipboard History": 14, "Keyboard Shortcuts": 16, "AI Chats": 16,
	checkForUpdatesTitle: 23, "Log in to Nevermind": 18, "Log out of Nevermind": 18}

// rootCommandScore is the score of a command at the empty root.
func rootCommandScore(title string) int {
	if score, ok := rootCommandScores[title]; ok {
		return score
	}
	return scoreRootCommand
}

// Each row function is one source of the root: it returns the rows for q, or the root rows for "". They run
// on the search goroutines over a captured rootInput; they touch the UI thread's state only inside actions,
// which run there.

// commandTitleScore is how well a command named title, with keywords, answers the lowercase query lq. The title scores
// first, and a query the keywords contain scores as an alias. phraseMatch limits the title to phrases and word prefixes.
func commandTitleScore(title, keywords string, phraseMatch bool, lq string) int {
	s := search.Score(title, lq)
	if phraseMatch && s < search.PhraseScoreFloor {
		s = 0
	}
	if s == 0 && len(lq) >= 2 && strings.Contains(strings.ToLower(keywords), lq) {
		s = scoreCommandAlias
	}
	return s
}

// commandRows lists the commands: the visible ones at the root, and the matches by title or keyword otherwise.
func commandRows(p *App, in *rootInput, q string) []search.Item {
	lq := strings.ToLower(q)
	var out []search.Item
	for _, cmd := range p.commandSlots() {
		if isBuiltInSlot(cmd) {
			out = append(out, p.builtInRootRows(cmd.builtIn, q, in)...)
			continue
		}
		cmd := cmd
		it := search.Item{Kind: search.KindCommands, Title: cmd.Title, Subtitle: cmd.Subtitle, Icon: cmd.Icon, Tone: cmd.Tone, LearnKey: "cmd:" + cmd.Title, Actions: []search.Action{{Title: cmd.Title, Run: p.trackedRun(cmd), Dismiss: cmd.Dismiss}}}
		if commandShortcutID(cmd) != "" {
			it = p.withShortcutActions(it, commandShortcutID(cmd), in.shortcuts)
		}
		it.LastUsed = commandLastUsed(cmd.Title, in)
		if lq == "" {
			if !cmd.Hidden && !(in.updatePrompt && cmd.Title == checkForUpdatesTitle) {
				it.Score = rootCommandScore(cmd.Title)
				if cmd.rootScore != 0 {
					it.Score = cmd.rootScore
				}
				out = append(out, it)
			}
			continue
		}
		s := commandTitleScore(cmd.Title, cmd.Keywords, cmd.PhraseMatch, lq)
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

// clipboardRows shows the copies at the empty root, up to rootClipboardLimit when the root list shows them, and
// otherwise the best text copies that contain the query, up to rootSearchClipboardLimit when the root search shows
// them. Pinned copies come first, then the newest. The history is read on the search goroutine; it locks its own state.
func clipboardRows(p *App, in *rootInput, q string) []search.Item {
	if q == "" {
		if !in.clipInRoot {
			return nil
		}
		return clipboardEntryRows(p, in, clipboard.Query{Limit: rootClipboardLimit}, func(int) int { return scoreRootClipboard })
	}
	if !in.clipSearch {
		return nil
	}
	return clipboardEntryRows(p, in, clipboard.Query{Text: q, Kinds: clipboardTextKinds, Limit: rootSearchClipboardLimit}, func(i int) int { return scoreClipboard - i })
}

// clipboardEntryRows turns the history entries of query into rows, scored by their position.
func clipboardEntryRows(p *App, in *rootInput, query clipboard.Query, score func(position int) int) []search.Item {
	entries := in.clip.history.Find(query)
	historyLen := in.clip.history.Len()
	out := make([]search.Item, 0, len(entries))
	for i, e := range entries {
		ci := p.clipItem(e, historyLen)
		ci.Kind = search.KindClipboard
		ci.Score = score(i)
		ci.LastUsed = clipboardLastUsed(e)
		out = append(out, ci)
	}
	return out
}

// conversationRows shows up to four recent AI chats, conversations and builder chats alike, filtered by title when a
// query is typed.
func conversationRows(p *App, in *rootInput, q string) []search.Item {
	lq := strings.ToLower(q)
	var out []search.Item
	for _, ch := range in.chats {
		if !chat.Listed(ch.kind) || ch.title == "" || len(out) >= 4 {
			continue
		}
		s := scoreRootConversation
		if lq != "" {
			if s = search.Score(ch.title, lq); s == 0 {
				continue
			}
		}
		id := ch.id
		title, subtitle := aichat.ContinueLabels(ch.kind, oneLine(ch.title))
		out = append(out, search.Item{Kind: search.KindConversations, Title: title, Subtitle: subtitle, Icon: aichat.ChatIcon(ch.kind), Tone: "yellow", TintTitle: true, Score: s,
			TiePriority: conversationTiePriority(ch.kind),
			LastUsed:    ch.lastUsed,
			Actions:     []search.Action{{Title: "Open", Run: func(h search.Host) { h.Push(aichat.ChatView(p.chatDeps(), id)) }}}})
	}
	return out
}

// automateRows offers Automate for a typed query that does not name a website: the AI builder writes a command for it.
// The web built-in answers the same query with the search and the website rows. Automate stays native, because the
// builder's write path is host-only (docs/extension-api.md, section 4). It is a query fallback, which an answer row
// leaves out.
func automateRows(p *App, _ *rootInput, q string) []search.Item {
	if q == "" || search.URLFromQuery(q) != "" {
		return nil
	}
	return []search.Item{{Kind: search.KindSearch, Title: `Automate "` + oneLine(q) + `"`, Subtitle: "Build a reusable command with AI", Icon: "zap", Tone: "yellow", Prominent: true, Score: scoreAutomate, QueryFallback: true,
		Actions: []search.Action{{Title: "Automate", Run: func(h search.Host) { h.Push(p.builderViewWith(q)) }}}}}
}

// conversationTiePriority puts builder chats after conversations at equal score and last use, as 1.x does
// (src/app/electron/search/action-ranking.ts:100-111).
func conversationTiePriority(kind chat.Kind) int {
	if kind == chat.KindBuilder {
		return -1
	}
	return 0
}

// clipboardLastUsed is the copy time of a clipboard row, the time 1.x sorts clipboard rows by
// (src/app/electron/clipboard/clipboard-history.ts:418).
func clipboardLastUsed(e clipboard.Entry) int64 {
	if e.CreatedAt.IsZero() {
		return 0
	}
	return e.CreatedAt.UnixMilli()
}

// commandLastUsed is the last use 1.x gives a root command: Clipboard History sorts just before the newest copy
// (src/app/electron/clipboard/clipboard-history.ts:804). Other commands have none.
func commandLastUsed(title string, in *rootInput) int64 {
	if title != "Clipboard History" || in.clip == nil {
		return 0
	}
	newest := in.clip.history.Find(clipboard.Query{Limit: 1})
	if len(newest) == 0 {
		return 0
	}
	if last := clipboardLastUsed(newest[0]); last != 0 {
		return last - 1
	}
	return 0
}
