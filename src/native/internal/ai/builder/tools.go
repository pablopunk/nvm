package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ai/chat"
)

// apiPageBytes bounds one page of the API reference a read returns, so a page fits one tool result.
const apiPageBytes = 12 << 10

// Tool names the model calls. They name what a tool does, as the builder's instructions refer to them.
const (
	readAPIToolName       = "read_extension_api"
	read1xMappingToolName = "read_1x_mapping"
	writeToolName         = "write_extension"
)

// readSchema is the arguments of read_extension_api: the byte offset of the page to read, from the start of the reference.
var readSchema = json.RawMessage(`{"type":"object","properties":{"offset":{"type":"integer"}},"additionalProperties":false}`)

// writeSchema is the arguments of write_extension: the manifest as JSON text, main.go, and any other Go files by name.
var writeSchema = json.RawMessage(`{"type":"object","properties":{` +
	`"manifest":{"type":"string","description":"The manifest as one JSON object with apiVersion, id and title, and optionally commandTitles."},` +
	`"source":{"type":"string","description":"The complete main.go source, with package main."},` +
	`"files":{"type":"object","description":"Other Go files of the extension, by file name such as helpers.go, each with package main."}` +
	`},"required":["manifest","source"],"additionalProperties":false}`)

// readArgs and writeArgs are the decoded arguments of the two tools.
type readArgs struct {
	Offset int `json:"offset"`
}

type writeArgs struct {
	Manifest string            `json:"manifest"`
	Source   string            `json:"source"`
	Files    map[string]string `json:"files"`
}

// readAPITool reads one page of the typed API reference. The reference is not in the instructions; the model reads the
// pages it needs.
func readAPITool() agent.Tool {
	return agent.Tool{
		Name:        readAPIToolName,
		Description: "Read one page of the full Nevermind extension API reference, with the prose behind each name. Call it again with the offset it names to read the next page.",
		Label:       "Reading the extension API",
		Schema:      readSchema,
		Run: func(_ context.Context, args json.RawMessage) (agent.Result, error) {
			var in readArgs
			if err := json.Unmarshal(args, &in); err != nil {
				return agent.Result{}, err
			}
			return apiPage(apiReference, in.Offset)
		},
	}
}

// read1xMappingTool reads the table of what each 1.x API name became, for converting older extensions.
func read1xMappingTool() agent.Tool {
	return agent.Tool{
		Name:        read1xMappingToolName,
		Description: "Read one page of the table of 1.x extension API names and their Go replacements. Use it when converting a 1.x extension.",
		Label:       "Reading the 1.x mapping",
		Schema:      readSchema,
		Run: func(_ context.Context, args json.RawMessage) (agent.Result, error) {
			var in readArgs
			if err := json.Unmarshal(args, &in); err != nil {
				return agent.Result{}, err
			}
			return apiPage(migration1x, in.Offset)
		},
	}
}

// writeTool is write_extension for chat chatID. It shows the extension's id in the transcript, never its source.
func (w *Workshop) writeTool(chatID string, reads *editReads, turn *chat.BuilderTurn) agent.Tool {
	return agent.Tool{
		Name:        writeToolName,
		Description: "Check, install and enable an extension. It reports the result: when it fails, fix the code and call it again.",
		Label:       "Writing extension",
		Schema:      writeSchema,
		Summarize:   summarizeWrite,
		Run: func(ctx context.Context, args json.RawMessage) (agent.Result, error) {
			return w.writeWithReads(ctx, chatID, args, reads, turn)
		},
	}
}

// summarizeWrite is the one-line summary of a write: the id of the extension in its manifest.
func summarizeWrite(args json.RawMessage) string {
	var in writeArgs
	if json.Unmarshal(args, &in) != nil {
		return ""
	}
	var manifest struct {
		ID string `json:"id"`
	}
	if json.Unmarshal([]byte(in.Manifest), &manifest) != nil {
		return ""
	}
	return manifest.ID
}

// apiPage returns the page of text that starts at offset, cut at a line end when one is near, with the offset of the
// next page. The page header says where it is in the reference.
func apiPage(text string, offset int) (agent.Result, error) {
	if offset < 0 || offset > len(text) {
		return agent.Result{}, fmt.Errorf("offset must be between 0 and %d", len(text))
	}
	end := pageEnd(text, offset)
	header := fmt.Sprintf("Extension API reference, bytes %d to %d of %d.", offset, end, len(text))
	if end < len(text) {
		header += fmt.Sprintf(" Read the next page with offset %d if you need it.", end)
	} else {
		header += " This is the end of the reference."
	}
	return agent.Result{Text: header + "\n\n" + text[offset:end], Outcome: "Read part of the API"}, nil
}

// pageEnd is where the page that starts at offset ends: apiPageBytes further, moved back to a line end and a rune
// boundary, or the end of the reference when that comes first.
func pageEnd(text string, offset int) int {
	if len(text)-offset <= apiPageBytes {
		return len(text)
	}
	end := offset + apiPageBytes
	if line := strings.LastIndexByte(text[offset:end], '\n'); line > 0 {
		end = offset + line + 1
	}
	for end > offset && !utf8.RuneStart(text[end]) {
		end--
	}
	return end
}
