// Package aichats is the built-in nevermind.ai-builder extension. It owns the AI Chats command and list, the continue
// rows at the root, the chat transcript, Remove Chat and the builder's prompt flow. It reaches the host only through the
// public extension API (ctx.AI().Chats(), ctx.AIBuilder(), ctx.Navigation()), so the host renders what it returns.
//
// Layer: ext. The package imports api, search and the standard library.
package aichats

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity. Its shortcut keys read extension:nevermind.ai-builder:<command id>.
const ID = "nevermind.ai-builder"

// CommandAIChats is the command that opens the AI Chats list, and cleanupCommandID the hidden job that removes expired
// conversations. Their IDs double as the refs their handlers answer to.
const (
	CommandAIChats   = "ai-chats"
	cleanupCommandID = "cleanup-expired-conversations"
)

// Handler IDs of the actions the built-in's rows and views run. Args carry the chat ID under argChatID.
const (
	opNewSubmit      = "new-submit"
	opOpenChat       = "open-chat"
	opOpenBuilder    = "open-builder"
	opSubmit         = "submit"
	opStop           = "stop"
	opTier           = "tier"
	opRefreshChat    = "refresh-chat"
	opRefreshHistory = "refresh-history"
	opTweak          = "tweak"
	argChatID        = "chatId"
	argExtensionID   = "extensionId"
)

// errChatGone answers an open of a chat the host no longer keeps.
var errChatGone = errors.New("this AI chat no longer exists")

// Extension is the AI Chats and AI builder built-in. It has no state: the host keeps the chats.
type Extension struct{}

var _ api.RowActionProvider = Extension{}

// New returns the built-in.
func New() Extension {
	return Extension{}
}

// Manifest identifies the built-in and declares the AI and ownership capabilities it uses.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion: "1.0", Identity: api.Identity{ID: ID, Title: "AI Builder"},
		Capabilities: []api.Capability{api.CapabilityAI, api.CapabilityExtensionsOwnership},
	}
}

// Commands lists the AI Chats command and the hidden cleanup job that runs at startup and hourly.
func (Extension) Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{aiChatsCommand(chatCount(ctx)), cleanupCommand()}, nil
}

// RootItems lists the four most recent conversations and builder chats as Continue rows.
func (Extension) RootItems(ctx api.Context) ([]api.Item, error) {
	return continueRows(ctx.AI().Chats().List(), "", rootChatLimit), nil
}

// SearchItems lists the Continue rows whose title holds the query, then Automate for a query that names no website.
func (Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	items := continueRows(ctx.AI().Chats().List(), q, rootChatLimit)
	if search.URLFromQuery(q) == "" {
		items = append(items, automateItem(ctx, q))
	}
	return items, nil
}

// RunAction answers the command, the views' rows and the chat actions. Each chat action runs on the host through the
// Next result, so the chat store does the work and the view the host shows comes back from the built-in.
func (Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case CommandAIChats:
		return ctx.Navigation().Push(historyView(ctx)), nil
	case cleanupCommandID:
		_, err := ctx.AIBuilder().CleanupExpiredConversations()
		return api.Result{}, err
	case opNewSubmit:
		return newChatSubmit(ctx, in), nil
	case opOpenChat, opOpenBuilder:
		return openChat(ctx, in.Ref.Args[argChatID])
	case opSubmit:
		return next(continueAction(in.Ref.Args[argChatID], in)), nil
	case opStop:
		return next(chatAction(api.ChatStop, in.Ref.Args[argChatID])), nil
	case opTier:
		return next(tierAction(in.Ref.Args[argChatID], in.Ref.Args["tier"])), nil
	case opRefreshChat:
		return refreshChat(ctx, in.Ref.Args[argChatID])
	case opRefreshHistory:
		return refreshHistory(ctx), nil
	case opTweak:
		return next(ctx.AIBuilder().TweakExtension(api.TweakRequest{ExtensionID: in.Ref.Args[argExtensionID]})), nil
	}
	return api.Result{}, fmt.Errorf("the AI builder built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// RowActions adds Tweak with AI to an extension's row, as 1.x's menu does for every extension.
func (Extension) RowActions(ctx api.Context, row api.ExtensionRow) ([]api.Action, error) {
	return []api.Action{ctx.AIBuilder().TweakExtension(api.TweakRequest{ExtensionID: row.ExtensionID})}, nil
}

// newChatSubmit starts a chat with the text the draft's composer sent. An empty send starts nothing, as Enter does on an
// empty composer of a new chat.
func newChatSubmit(ctx api.Context, in api.Invocation) api.Result {
	text := in.Form["message"].Text
	if text == "" {
		return api.Result{}
	}
	return next(ctx.AI().Chats().New(text))
}

// next returns a result that runs action on the host once the handler returns.
func next(action api.Action) api.Result {
	return api.Result{Next: &action}
}

// openChat shows the transcript of the chat with id, or fails when the host no longer keeps it.
func openChat(ctx api.Context, id string) (api.Result, error) {
	detail, ok := ctx.AI().Chats().Get(id)
	if !ok {
		return api.Result{}, errChatGone
	}
	return ctx.Navigation().Push(chatView(ctx, detail)), nil
}

// refreshChat returns the chat's transcript as it is now, which is how a streaming reply shows its next words.
func refreshChat(ctx api.Context, id string) (api.Result, error) {
	detail, ok := ctx.AI().Chats().Get(id)
	if !ok {
		return api.Result{}, errChatGone
	}
	return api.Result{View: chatView(ctx, detail)}, nil
}

// refreshHistory replaces the list's rows, so a removed chat drops out of it.
func refreshHistory(ctx api.Context) api.Result {
	return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: historyItems(ctx)}}
}

// chatCount is the number of chats the host keeps, which the AI Chats subtitle names.
func chatCount(ctx api.Context) int {
	return len(ctx.AI().Chats().List())
}

// automateItem is the Automate row for query q: the AI builder writes a command for it.
func automateItem(ctx api.Context, q string) api.Item {
	title := `Automate "` + oneLine(q) + `"`
	start := ctx.AIBuilder().StartChat(q, title)
	return api.Item{
		ID: "automate", Title: title, Subtitle: "Build a reusable command with AI", Icon: "zap",
		Section: search.KindSearch, QueryFallback: true, Score: scoreAutomate,
		Appearance: &api.Appearance{Foreground: "yellow", Background: "accent"},
		Primary:    &start,
	}
}

// chatAction is a host chat action on the chat with id.
func chatAction(op, id string) api.Action {
	return api.Action{Kind: api.ActionChat, Op: op, Chat: &api.ChatAction{ChatID: id}}
}

// tierAction changes the tier of the chat with id to the tier named by tier.
func tierAction(id, tier string) api.Action {
	return api.Action{Kind: api.ActionChat, Op: api.ChatTier, Chat: &api.ChatAction{ChatID: id, Tier: api.ModelTier(tier)}}
}

// continueAction sends the composer's text and pictures to the chat with id. Empty text with no pictures allows a
// waiting tool call, as Enter on an empty composer does.
func continueAction(id string, in api.Invocation) api.Action {
	return api.Action{Kind: api.ActionChat, Op: api.ChatContinue, Chat: &api.ChatAction{
		ChatID: id, Message: in.Form["message"].Text, Images: in.Images,
	}}
}

// cleanupCommand is the hidden background job that removes expired conversations.
func cleanupCommand() api.Command {
	return api.Command{
		ID: cleanupCommandID, Title: "Clean Up Expired AI Conversations", Mode: api.ModeBackground,
		Placement: []api.Placement{api.PlacementHidden},
		Triggers:  []api.Trigger{{Type: api.TriggerStartup, DelayMs: cleanupStartupDelayMs}, {Type: api.TriggerInterval, Every: "1h"}},
	}
}

// aiChatsCommand is the AI Chats command, whose subtitle counts the saved chats.
func aiChatsCommand(count int) api.Command {
	return api.Command{
		ID: CommandAIChats, Title: "AI Chats", Subtitle: chatsSubtitle(count), Icon: "sparkles", Score: scoreAIChatsCommand,
		Keywords:   strings.Fields("ai chat assistant gpt llm ask conversations history"),
		Appearance: &api.Appearance{Foreground: "purple"},
	}
}
