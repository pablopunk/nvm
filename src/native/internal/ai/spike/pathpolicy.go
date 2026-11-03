package spike

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

const (
	unreadablePathReason = "the path argument could not be read"
	withheldSearchOutput = "[search output withheld: its path argument could not be read]"
	searchToolGrep       = "grep"
)

// PathPolicy guards the path argument of the file tools and redacts credential lines from their search output. It
// is the port of the 1.x tool_call and tool_result handlers. A path it cannot read is refused, not allowed: 1.x
// fails open on malformed arguments, and the spike does not.
type PathPolicy struct {
	guard       *Guard
	cwd         string
	pathTools   NameAllowlist
	searchTools NameAllowlist
}

// NewPathPolicy guards read, grep, find and ls, the four tools 1.x guards, and redacts grep, find and ls output.
func NewPathPolicy(guard *Guard, cwd string) PathPolicy {
	return PathPolicy{
		guard:       guard,
		cwd:         cwd,
		pathTools:   AllowNames("read", "grep", "find", "ls"),
		searchTools: AllowNames("grep", "find", "ls"),
	}
}

// Decide refuses a guarded call whose path is a credential location, an env file, or outside the configured roots.
func (p PathPolicy) Decide(call ToolCall, _ Tool) Decision {
	if !p.pathTools[call.Name] {
		return Decision{Allow: true}
	}
	raw, ok := requestedPath(call.Arguments, p.cwd)
	if !ok {
		return Decision{Reason: unreadablePathReason}
	}
	return p.guard.Check(raw, p.cwd)
}

// FilterResult removes the lines of search output that name a credential path, then leaves the rest unchanged.
func (p PathPolicy) FilterResult(call ToolCall, _ Tool, output string) string {
	if !p.searchTools[call.Name] {
		return output
	}
	raw, ok := requestedPath(call.Arguments, p.cwd)
	if !ok {
		return withheldSearchOutput
	}
	return p.guard.RedactSearchOutput(call.Name, p.guard.Resolve(raw, p.cwd), output)
}

// requestedPath returns the call's path argument, or the working directory when the call names none, as 1.x does.
func requestedPath(arguments, cwd string) (string, bool) {
	var args struct {
		Path *string `json:"path"`
	}
	if err := json.Unmarshal([]byte(normalizedArguments(arguments)), &args); err != nil {
		return "", false
	}
	if args.Path == nil {
		return cwd, true
	}
	return *args.Path, true
}

// RedactSearchOutput drops every line of a search result whose path lies in a credential location. searchRoot is the
// canonical directory the search ran in.
func (g *Guard) RedactSearchOutput(tool, searchRoot, output string) string {
	lines := strings.Split(output, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if !g.isCredentialSearchLine(tool, line, searchRoot) {
			kept = append(kept, line)
		}
	}
	if len(kept) == len(lines) {
		return output
	}
	return strings.Join(kept, "\n")
}

func (g *Guard) isCredentialSearchLine(tool, line, searchRoot string) bool {
	candidates := searchLinePathCandidates(tool, line)
	for _, candidate := range candidates {
		if pathNamesEnvFile(candidate) {
			return true
		}
	}
	policy := g.canonical().policy
	for _, candidate := range candidates {
		if IsCredentialPath(g.resolveSearchCandidate(candidate, searchRoot), policy) {
			return true
		}
	}
	return false
}

func (g *Guard) resolveSearchCandidate(candidate, searchRoot string) string {
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(searchRoot, candidate)
	}
	return g.canonicalizeExistingAncestor(filepath.Clean(candidate))
}

func searchLinePathCandidates(tool, line string) []string {
	if tool == searchToolGrep {
		return grepLinePathCandidates(line)
	}
	trimmed := strings.TrimSuffix(line, "/")
	if trimmed == "" {
		return nil
	}
	return []string{trimmed}
}

// grepLinePathCandidates returns every prefix of a grep line that could be its path. A line is "path:N: text" for a
// match and "path-N- text" for context, and the path may itself contain ":N:", so each boundary yields a candidate.
func grepLinePathCandidates(line string) []string {
	var candidates []string
	for index := 1; index < len(line); index++ {
		if (line[index] == ':' || line[index] == '-') && isGrepLineNumberBoundary(line, index) {
			candidates = append(candidates, line[:index])
		}
	}
	return candidates
}

// isGrepLineNumberBoundary reports whether line has [:-] digits [:-] space starting at index. It is the lookahead
// 1.x uses, written out because Go regular expressions have no lookahead.
func isGrepLineNumberBoundary(line string, index int) bool {
	cursor := index + 1
	for cursor < len(line) && line[cursor] >= '0' && line[cursor] <= '9' {
		cursor++
	}
	if cursor == index+1 || cursor+1 >= len(line) {
		return false
	}
	return (line[cursor] == ':' || line[cursor] == '-') && line[cursor+1] == ' '
}
