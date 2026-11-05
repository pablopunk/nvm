// Package protocol is the line-delimited JSON exchange between the Nevermind host and an extension child
// process. Each request and each response is one line on the child's stdin and stdout. Only wire types from
// the api package cross the boundary, so no code, handle or closure does.
//
// Layer: ext. The package imports api and the standard library only.
package protocol

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Methods the host may ask a child to run. Each maps to one api interface.
const (
	MethodCommands = "commands" // CommandProvider.Commands
	MethodRoot     = "root"     // RootProvider.RootItems
	MethodSearch   = "search"   // SearchProvider.SearchItems, with Query
	MethodRun      = "run"      // ActionHandler.RunAction, with Invocation
	MethodLoad     = "load"     // DataLoader.LoadItems, with Invocation.Ref.ID as the loader

	MethodMenuBars        = "menubars"         // MenuBarProvider.MenuBars
	MethodMenuBarTriggers = "menubar-triggers" // MenuBarProvider.MenuBarTriggers
)

// Host services an extension may call while a request runs. The child sends a ServiceCall and blocks until the
// host answers with a ServiceReply carrying the same CallID. The host decides what each call may do.
const (
	ServiceStorage    = "storage"
	ServiceSecrets    = "secrets"
	ServiceSettings   = "settings"
	ServiceLogs       = "logs"
	ServiceText       = "text"
	ServiceNetwork    = "network"
	ServiceDrafts     = "drafts"
	ServiceViews      = "views"
	ServiceIndicators = "indicators"
	ServiceScheduler  = "scheduler"
	ServiceAI         = "ai"
	ServiceBuilder    = "builder"
	ServiceBundle     = "bundle"
	ServiceDictation  = "dictation"
)

// ChildFlag is the first argument that makes the binary an extension child. The arguments that follow are the
// extension source path, the address-space limit in bytes (0 for none) and the identity as JSON.
const ChildFlag = "--nvm-ext-child"

// MaxLineBytes bounds one protocol line in either direction. A longer line is a failed call, not a
// partial read: the host kills the child.
const MaxLineBytes = 1 << 20

// Ready is the child's first line. It reports whether the extension source loaded. A child that sends Ready
// with OK false exits right after. The manifest is never read from the child: the host reviews extension.json.
type Ready struct {
	OK      bool         `json:"ok"`
	Failure *api.Failure `json:"failure,omitempty"`
	// Methods lists the protocol methods the source implements, so the host knows a provider exists without a call.
	Methods []string `json:"methods,omitempty"`
}

// Request is one call from the host. ID is echoed in the response, so a late answer is never mistaken
// for the next one.
type Request struct {
	ID         uint64          `json:"id"`
	Method     string          `json:"method"`
	Query      string          `json:"query,omitempty"`
	Invocation *api.Invocation `json:"invocation,omitempty"`
}

// Response answers one Request. Data holds the method's JSON result when Failure is nil.
type Response struct {
	ID      uint64          `json:"id"`
	Data    json.RawMessage `json:"data,omitempty"`
	Failure *api.Failure    `json:"failure,omitempty"`
}

// ServiceCall is one call from the child to a host service. A child line is a ServiceCall when Service is set;
// every other child line is a Response. CallID is chosen by the child and echoed in the reply.
type ServiceCall struct {
	CallID  uint64          `json:"callId"`
	Service string          `json:"service"`
	Op      string          `json:"op"`
	Args    json.RawMessage `json:"args,omitempty"`
}

// ServiceReply answers one ServiceCall. Data holds the JSON result when Failure is nil. The host writes it on the
// child's stdin, so a host line is a ServiceReply when ReplyTo is set and a Request otherwise.
type ServiceReply struct {
	ReplyTo uint64          `json:"replyTo"`
	Data    json.RawMessage `json:"data,omitempty"`
	Failure *api.Failure    `json:"failure,omitempty"`
	// WaitedForUser is set by the host and never sent: the call waited for a palette answer, so the request budget
	// restarts after it.
	WaitedForUser bool `json:"-"`
}

// ParseServiceCall reports whether a child line is a service call and returns it.
func ParseServiceCall(line []byte) (ServiceCall, bool) {
	var call ServiceCall
	if err := json.Unmarshal(line, &call); err != nil || call.Service == "" {
		return ServiceCall{}, false
	}
	return call, true
}

// IsServiceReply reports whether a host line answers a service call rather than being a request.
func IsServiceReply(line []byte) bool {
	var probe struct {
		ReplyTo *uint64 `json:"replyTo"`
	}
	return json.Unmarshal(line, &probe) == nil && probe.ReplyTo != nil
}
