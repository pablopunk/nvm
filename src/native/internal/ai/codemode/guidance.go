package codemode

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/program"
)

// MaxGuidanceBytes bounds the text the model reads about run_program in every request: the contract, the generated api
// reference and the examples. It is the plan's estimate of about 900 tokens, with room for the next services.
const MaxGuidanceBytes = 4 << 10

// contract is the part of the guidance that is not the api: what a program is, what it can use, and how it ends.
const contract = `Runs a short Go program that reads or changes the user's notes. Use it for any request about their notes.

Send only the statements of func Main(ctx api.Context) (any, error): no package line, imports or func. ctx is the API. ` +
	`You may use these packages by name: PACKAGES. Nothing else exists: no files, network, os, goroutines, channels or generics. ` +
	`End with return value, nil or return nil, err. The JSON of the value comes back to you, so return ids, counts and short excerpts, not every note. Printing is discarded. ` +
	`Limits: 8 KiB, 50 calls, 20 s of computing.

The host asks the user before it deletes a note. If they decline, the call returns an error: stop and say so. Creating, editing and reading notes do not ask. ` +
	`The result lists what the host really did; report that, not what you meant to do. Update returns true when the note changed since you read it, and then saves nothing. ` +
	`The interpreter cannot return a computed bool directly: write ok := a < b, then return ok, nil.

API:
`

// examples are programs the tests run against a real notes store, so the guidance cannot describe a call that fails.
const examples = `
Create:
note, err := ctx.Notes().CreateWith("# Packing list\n- socks")
if err != nil { return nil, err }
return note.ID, nil

Append to a note found by words:
found, err := ctx.Notes().Search("shopping", 1)
if err != nil || len(found) == 0 { return "no such note", err }
note, _, err := ctx.Notes().Get(found[0].ID)
if err != nil { return nil, err }
_, conflict, err := ctx.Notes().Update(note.ID, note.Markdown+"\n- milk", note.UpdatedAt)
return conflict, err

Delete (asks the user):
found, err := ctx.Notes().Search("old ideas", 1)
if err != nil || len(found) == 0 { return "no such note", err }
return found[0].ID, ctx.Notes().Delete(found[0].ID)
`

// guidance is the description of run_program.
func guidance() string {
	return strings.Replace(contract, "PACKAGES", packageList(), 1) + apiSubset + examples
}

func packageList() string {
	names := make([]string, 0, len(program.Packages))
	for _, importPath := range program.Packages {
		names = append(names, program.PackageName(importPath))
	}
	return strings.Join(names, ", ")
}
