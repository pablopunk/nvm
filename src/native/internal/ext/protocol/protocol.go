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
)

// MaxLineBytes bounds one protocol line in either direction. A longer line is a failed call, not a
// partial read: the host kills the child.
const MaxLineBytes = 1 << 20

// Ready is the child's first line. It reports whether the extension source compiled and its manifest
// read cleanly. A child that sends Ready with OK false exits right after.
type Ready struct {
	OK       bool          `json:"ok"`
	Manifest *api.Manifest `json:"manifest,omitempty"`
	Failure  *api.Failure  `json:"failure,omitempty"`
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
