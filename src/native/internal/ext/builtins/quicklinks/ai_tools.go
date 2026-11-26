package quicklinks

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Names of the tools the quicklinks built-in gives Nevermind's AI.
const (
	toolList   = "list-quicklinks"
	toolCreate = "create-quicklink"
	toolUpdate = "update-quicklink"
	toolDelete = "delete-quicklink"
)

// aiTools are the quicklinks tools. A link is named by its keyword; the six defaults are listed but never changed.
func aiTools() []api.AITool {
	return []api.AITool{
		{Name: toolList, Class: api.AIToolRead,
			Description: "Lists the user's quicklinks, then the built-in ones, which cannot change. Typing \"keyword words\" opens url with the words in place of {query}.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)},
		{Name: toolCreate, Class: api.AIToolWrite,
			Description: "Creates a quicklink. keyword is one word no other quicklink has; url is http(s) with {query} where the search text goes.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"keyword":{"type":"string"},"url":{"type":"string"}},"required":["name","keyword","url"],"additionalProperties":false}`)},
		{Name: toolUpdate, Class: api.AIToolWrite,
			Description: "Changes the user's quicklink with keyword: its name, its keyword (newKeyword) or its url. A field left out keeps its value.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"keyword":{"type":"string"},"name":{"type":"string"},"newKeyword":{"type":"string"},"url":{"type":"string"}},"required":["keyword"],"additionalProperties":false}`)},
		{Name: toolDelete, Class: api.AIToolDestructive,
			Description: "Deletes the user's quicklink with keyword.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"keyword":{"type":"string"}},"required":["keyword"],"additionalProperties":false}`)},
	}
}

// linkToolArgs are the arguments of every quicklinks tool. A pointer is a field the call may leave out.
type linkToolArgs struct {
	Keyword    string  `json:"keyword"`
	Name       *string `json:"name"`
	NewKeyword *string `json:"newKeyword"`
	URL        *string `json:"url"`
}

// toolLink is a link as the list tool returns it.
type toolLink struct {
	Link
	BuiltIn bool `json:"builtIn,omitempty"`
}

// linkChange is a create, update or delete a tool call plans against the stored links: the links after it, and the
// link before and after the change.
type linkChange struct {
	links  []Link
	before Link
	after  Link
}

// CallAITool runs one quicklinks tool.
func (e Extension) CallAITool(ctx api.Context, call api.AIToolCall) (any, error) {
	if call.Name == toolList {
		return e.toolLinks(ctx), nil
	}
	change, err := e.planLinkChange(ctx, call)
	if err != nil {
		return nil, err
	}
	if err := ctx.Storage().Set(storageKey, change.links); err != nil {
		return nil, err
	}
	if call.Name == toolDelete {
		return nil, nil
	}
	return change.after, nil
}

// DescribeAITool says what a quicklinks tool call will do. A change that would fail, such as a keyword another link has,
// is refused here, before the user is asked.
func (e Extension) DescribeAITool(ctx api.Context, call api.AIToolCall) (string, error) {
	if call.Name == toolList {
		return "List the quicklinks", nil
	}
	change, err := e.planLinkChange(ctx, call)
	if err != nil {
		return "", err
	}
	switch call.Name {
	case toolCreate:
		return fmt.Sprintf("Create the quicklink “%s” (keyword “%s”) for %s", change.after.Name, change.after.Keyword, change.after.URL), nil
	case toolUpdate:
		return fmt.Sprintf("Change the quicklink “%s” (keyword “%s”): %s", change.before.Name, change.before.Keyword, linkChanges(change.before, change.after)), nil
	}
	return fmt.Sprintf("Delete the quicklink “%s” (keyword “%s”) for %s", change.before.Name, change.before.Keyword, change.before.URL), nil
}

// planLinkChange checks a create, update or delete against the stored links and returns the links it leaves.
func (e Extension) planLinkChange(ctx api.Context, call api.AIToolCall) (linkChange, error) {
	var args linkToolArgs
	if len(call.Args) > 0 && json.Unmarshal(call.Args, &args) != nil {
		return linkChange{}, linkRefusal("the arguments of %s do not fit it", call.Name)
	}
	links := e.links(ctx)
	switch call.Name {
	case toolCreate:
		link := cleanLink(Link{Name: valueOr(args.Name, ""), Keyword: args.Keyword, URL: valueOr(args.URL, "")})
		return linkChange{links: append(links, link), after: link}, checkLink(links, link, "")
	case toolUpdate:
		return updatedLinks(links, args)
	case toolDelete:
		i, err := userLinkIndex(links, args.Keyword)
		if err != nil {
			return linkChange{}, err
		}
		return linkChange{links: slices.Delete(slices.Clone(links), i, i+1), before: links[i]}, nil
	}
	return linkChange{}, linkRefusal("quicklinks has no tool %q", call.Name)
}

func updatedLinks(links []Link, args linkToolArgs) (linkChange, error) {
	i, err := userLinkIndex(links, args.Keyword)
	if err != nil {
		return linkChange{}, err
	}
	before := links[i]
	after := cleanLink(Link{Name: valueOr(args.Name, before.Name), Keyword: valueOr(args.NewKeyword, before.Keyword), URL: valueOr(args.URL, before.URL)})
	changed := slices.Clone(links)
	changed[i] = after
	return linkChange{links: changed, before: before, after: after}, checkLink(links, after, before.Keyword)
}

// checkLink refuses a link that lacks a part, or whose keyword another link has; own is the keyword the link had.
func checkLink(links []Link, link Link, own string) error {
	if !isValidLink(link) || strings.ContainsAny(link.Keyword, " \t") {
		return linkRefusal("a quicklink needs a name, a one-word keyword and an http(s) URL")
	}
	for _, other := range append(slices.Clone(links), defaultLinks...) {
		if strings.EqualFold(other.Keyword, link.Keyword) && !strings.EqualFold(other.Keyword, own) {
			return linkRefusal("keyword “%s” is already used by the quicklink “%s”; choose another keyword", link.Keyword, other.Name)
		}
	}
	return nil
}

// userLinkIndex finds the user's link with keyword. A default link cannot change.
func userLinkIndex(links []Link, keyword string) (int, error) {
	if i := slices.IndexFunc(links, func(l Link) bool { return strings.EqualFold(l.Keyword, keyword) }); i >= 0 {
		return i, nil
	}
	if slices.ContainsFunc(defaultLinks, func(l Link) bool { return strings.EqualFold(l.Keyword, keyword) }) {
		return -1, linkRefusal("“%s” is a built-in quicklink, which cannot change", keyword)
	}
	return -1, linkRefusal("no quicklink has the keyword “%s”; list them first", keyword)
}

// linkChanges names what an update changes.
func linkChanges(before, after Link) string {
	var changes []string
	if after.Name != before.Name {
		changes = append(changes, fmt.Sprintf("name to “%s”", after.Name))
	}
	if after.Keyword != before.Keyword {
		changes = append(changes, fmt.Sprintf("keyword to “%s”", after.Keyword))
	}
	if after.URL != before.URL {
		changes = append(changes, "URL to "+after.URL)
	}
	if len(changes) == 0 {
		return "nothing changes"
	}
	return strings.Join(changes, ", ")
}

func (e Extension) toolLinks(ctx api.Context) []toolLink {
	var out []toolLink
	for _, link := range e.links(ctx) {
		out = append(out, toolLink{Link: link})
	}
	for _, link := range defaultLinks {
		out = append(out, toolLink{Link: link, BuiltIn: true})
	}
	return out
}

func cleanLink(link Link) Link {
	return Link{Name: strings.TrimSpace(link.Name), Keyword: strings.TrimSpace(link.Keyword), URL: strings.TrimSpace(link.URL)}
}

func valueOr(value *string, otherwise string) string {
	if value == nil {
		return otherwise
	}
	return *value
}

func linkRefusal(format string, args ...any) error {
	return fmt.Errorf(format+" (%w)", append(args, api.ErrInvalidInput)...)
}
