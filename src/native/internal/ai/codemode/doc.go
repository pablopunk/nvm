// Package codemode lets an AI conversation drive Nevermind through one tool, run_program. The model writes the body of a
// short Go program against the Extension API; the host checks it (package program), runs it in a throwaway child of the
// extension runtime, and answers every service call the program makes through a gate. The gate is the security
// boundary: it lives in Go, outside the model and outside the sidecar, and decides for each call whether it may run,
// whether the user must allow it first, and what the model is told about it. Nothing the program or the model says
// changes that decision.
//
// Layer: ai. It imports the agent package, the extension runtime (supervisor, services, protocol) and the notes store.
package codemode

//go:generate go run ../builder/apiref/gen -api ../../ext/api -subset Notes -package codemode -var apiSubset -out reference_gen.go
