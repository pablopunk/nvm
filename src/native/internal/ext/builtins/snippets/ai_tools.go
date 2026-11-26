package snippets

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Names of the tools the snippets built-in gives Nevermind's AI.
const (
	toolSearch = "search-snippets"
	toolCreate = "create-snippet"
	toolUpdate = "update-snippet"
	toolDelete = "delete-snippet"
)

// maxToolTextRunes bounds the text of one snippet a search returns to the AI.
const maxToolTextRunes = 2000

// aiTools are the snippets tools: a search reads, a create or update is easily undone, and a delete asks every time.
func aiTools() []api.AITool {
	return []api.AITool{
		{Name: toolSearch, Class: api.AIToolRead,
			Description: "Lists the user's snippets whose name, keyword or text holds every word of query; an empty query lists all.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"additionalProperties":false}`)},
		{Name: toolCreate, Class: api.AIToolWrite,
			Description: "Creates a snippet. keyword is one word typed at the root to paste it, optional and unique; text may hold {clipboard} and {date}.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"keyword":{"type":"string"},"text":{"type":"string"}},"required":["name","text"],"additionalProperties":false}`)},
		{Name: toolUpdate, Class: api.AIToolWrite,
			Description: "Changes the name, keyword or text of the snippet with id; a field left out keeps its value, and keyword \"\" removes it.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"keyword":{"type":"string"},"text":{"type":"string"}},"required":["id"],"additionalProperties":false}`)},
		{Name: toolDelete, Class: api.AIToolDestructive,
			Description: "Deletes the snippet with id.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"],"additionalProperties":false}`)},
	}
}

// snippetToolArgs are the arguments of every snippets tool. A pointer is a field the call may leave out.
type snippetToolArgs struct {
	Query   string  `json:"query"`
	ID      string  `json:"id"`
	Name    *string `json:"name"`
	Keyword *string `json:"keyword"`
	Text    *string `json:"text"`
}

// toolSnippet is a snippet as a tool returns it.
type toolSnippet struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Keyword string `json:"keyword,omitempty"`
	Text    string `json:"text"`
}

// CallAITool runs one snippets tool.
func (e Extension) CallAITool(ctx api.Context, call api.AIToolCall) (any, error) {
	args, err := snippetArgsOf(call)
	if err != nil {
		return nil, err
	}
	switch call.Name {
	case toolSearch:
		return toolSnippetsOf(searchSnippets(e.listSnippets(ctx), args.Query)), nil
	case toolCreate:
		created, err := e.createSnippet(ctx, e.createInput(args))
		return toolSnippetOf(created), toolRefusal(err)
	case toolUpdate:
		input, err := e.updateInput(ctx, args)
		if err != nil {
			return nil, err
		}
		updated, err := e.updateSnippet(ctx, args.ID, input)
		return toolSnippetOf(updated), toolRefusal(err)
	case toolDelete:
		return nil, toolRefusal(e.deleteSnippet(ctx, args.ID))
	}
	return nil, unknownTool(call.Name)
}

// DescribeAITool says what a snippets tool call will do, from the stored snippets. A change that would fail, such as a
// keyword another snippet has or an id no snippet has, is refused here, before the user is asked.
func (e Extension) DescribeAITool(ctx api.Context, call api.AIToolCall) (string, error) {
	args, err := snippetArgsOf(call)
	if err != nil {
		return "", err
	}
	all := e.listSnippets(ctx)
	switch call.Name {
	case toolSearch:
		if args.Query == "" {
			return "List every snippet", nil
		}
		return fmt.Sprintf("Search snippets for “%s”", args.Query), nil
	case toolCreate:
		input, err := normalizeInput(e.createInput(args))
		if err == nil {
			err = checkKeyword(all, input.Keyword, "")
		}
		return fmt.Sprintf("Create the snippet “%s”%s", input.Name, keywordText(input.Keyword)), toolRefusal(err)
	case toolUpdate:
		return e.describeUpdate(ctx, all, args)
	case toolDelete:
		sn, found := findSnippet(all, args.ID)
		if !found {
			return "", toolRefusal(errNotFound)
		}
		return fmt.Sprintf("Delete the snippet “%s”%s", sn.Name, keywordText(sn.Keyword)), nil
	}
	return "", unknownTool(call.Name)
}

func (e Extension) describeUpdate(ctx api.Context, all []Snippet, args snippetToolArgs) (string, error) {
	before, found := findSnippet(all, args.ID)
	if !found {
		return "", toolRefusal(errNotFound)
	}
	input, err := e.updateInput(ctx, args)
	if err != nil {
		return "", err
	}
	after, err := normalizeInput(input)
	if err == nil {
		err = checkKeyword(all, after.Keyword, args.ID)
	}
	return fmt.Sprintf("Change the snippet “%s”: %s", before.Name, snippetChanges(before, after)), toolRefusal(err)
}

// snippetChanges names what an update changes, in the order the form shows the fields.
func snippetChanges(before Snippet, after Input) string {
	var changes []string
	if after.Name != before.Name {
		changes = append(changes, fmt.Sprintf("name to “%s”", after.Name))
	}
	if after.Keyword != before.Keyword {
		changes = append(changes, "keyword"+keywordChange(after.Keyword))
	}
	if after.Text != before.Text {
		changes = append(changes, "its text")
	}
	if len(changes) == 0 {
		return "nothing changes"
	}
	return strings.Join(changes, ", ")
}

func keywordChange(keyword string) string {
	if keyword == "" {
		return " removed"
	}
	return fmt.Sprintf(" to “%s”", keyword)
}

func keywordText(keyword string) string {
	if keyword == "" {
		return ""
	}
	return fmt.Sprintf(" (keyword “%s”)", keyword)
}

func (Extension) createInput(args snippetToolArgs) Input {
	return Input{Name: valueOr(args.Name, ""), Keyword: valueOr(args.Keyword, ""), Text: valueOr(args.Text, "")}
}

// updateInput is the snippet's fields with the ones the call names replaced.
func (e Extension) updateInput(ctx api.Context, args snippetToolArgs) (Input, error) {
	sn, found := findSnippet(e.listSnippets(ctx), args.ID)
	if !found {
		return Input{}, toolRefusal(errNotFound)
	}
	return Input{Name: valueOr(args.Name, sn.Name), Keyword: valueOr(args.Keyword, sn.Keyword), Text: valueOr(args.Text, sn.Text)}, nil
}

// searchSnippets returns the snippets whose name, keyword or text holds every word of query, by name.
func searchSnippets(all []Snippet, query string) []Snippet {
	words := strings.Fields(strings.ToLower(query))
	var found []Snippet
	for _, sn := range sortedByName(all) {
		text := strings.ToLower(sn.Name + " " + sn.Keyword + " " + sn.Text)
		if holdsEveryWord(text, words) {
			found = append(found, sn)
		}
	}
	return found
}

func holdsEveryWord(text string, words []string) bool {
	for _, word := range words {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

func toolSnippetsOf(all []Snippet) []toolSnippet {
	out := make([]toolSnippet, 0, len(all))
	for _, sn := range all {
		out = append(out, toolSnippetOf(sn))
	}
	return out
}

func toolSnippetOf(sn Snippet) toolSnippet {
	text := sn.Text
	if runes := []rune(text); len(runes) > maxToolTextRunes {
		text = string(runes[:maxToolTextRunes]) + "…"
	}
	return toolSnippet{ID: sn.ID, Name: sn.Name, Keyword: sn.Keyword, Text: text}
}

func snippetArgsOf(call api.AIToolCall) (snippetToolArgs, error) {
	var args snippetToolArgs
	if len(call.Args) > 0 && json.Unmarshal(call.Args, &args) != nil {
		return args, fmt.Errorf("the arguments of %s do not fit it: %w", call.Name, api.ErrInvalidInput)
	}
	return args, nil
}

func valueOr(value *string, otherwise string) string {
	if value == nil {
		return otherwise
	}
	return *value
}

// toolRefusal marks a refusal the user can act on, such as a taken keyword, as invalid input, so the AI reads its reason.
func toolRefusal(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w (%w)", err, api.ErrInvalidInput)
}

func unknownTool(name string) error {
	return fmt.Errorf("snippets has no tool %q: %w", name, api.ErrInvalidInput)
}
