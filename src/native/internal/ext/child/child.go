// Package child is the extension process. The host starts the Nevermind binary again with Flag, the extension
// source path, a memory limit and the extension's identity as JSON. This package interprets the source with
// Yaegi and answers line-delimited requests on stdin with responses on stdout (see package protocol). Extension
// code can crash, hang or exhaust memory in this process only; the host kills it at the deadline.
//
// Extension source imports only the standard library. Its methods take and return plain JSON-shaped values,
// because the host's types cannot be exported to the interpreter without a generated table that the file-size
// ratchet forbids (see docs/extension-runtime-spike.md and docs/extension-api.md).
//
// Layer: ext. The package imports api, protocol and the Yaegi interpreter.
package child

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// Main runs the child when args ask for it and reports the exit code with true. For any other command line
// it returns false and the caller starts the app as usual. The child's stdout carries the protocol, so
// os.Stdout is pointed at stderr for the extension's own printing.
func Main(args []string) (exitCode int, handled bool) {
	if len(args) < 4 || args[1] != protocol.ChildFlag {
		return 0, false
	}
	limit, err := strconv.ParseUint(args[3], 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "extension child: bad memory limit: %v\n", err)
		return 2, true
	}
	identity, err := parseIdentity(argOr(args, 4))
	if err != nil {
		fmt.Fprintf(os.Stderr, "extension child: bad identity: %v\n", err)
		return 2, true
	}
	protocolOut := os.Stdout
	os.Stdout = os.Stderr
	return serve(args[2], limit, identity, os.Stdin, protocolOut), true
}

func argOr(args []string, index int) string {
	if index < len(args) {
		return args[index]
	}
	return "{}"
}

func parseIdentity(text string) (api.Identity, error) {
	var identity api.Identity
	err := json.Unmarshal([]byte(text), &identity)
	return identity, err
}

// serve applies the memory limit, loads the extension, announces it, and answers requests until stdin ends.
func serve(sourcePath string, memoryLimit uint64, identity api.Identity, in io.Reader, out io.Writer) int {
	if memoryLimit > 0 {
		if err := applyMemoryLimit(memoryLimit); err != nil {
			fmt.Fprintf(os.Stderr, "extension child: memory limit not applied: %v\n", err)
		}
	}
	ext, err := load(sourcePath, identity)
	if err != nil {
		failure := api.FailureOf(err)
		failure.Message = truncate(failure.Message)
		writeLine(out, protocol.Ready{OK: false, Failure: &failure})
		return 1
	}
	writeLine(out, protocol.Ready{OK: true})
	return answerRequests(ext, in, out)
}

// answerRequests reads one request per line and writes one response per line, in order.
func answerRequests(ext *extension, in io.Reader, out io.Writer) int {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64<<10), protocol.MaxLineBytes)
	for scanner.Scan() {
		var req protocol.Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			writeLine(out, protocol.Response{Failure: &api.Failure{Code: api.CodeInvalid, Message: "request is not valid JSON"}})
			continue
		}
		writeLine(out, ext.answer(req))
	}
	return 0
}

// writeLine marshals v and writes it as one line. A write error ends the child: the host has gone.
func writeLine(out io.Writer, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		data = []byte(`{"failure":{"code":"internal","message":"response cannot be encoded"}}`)
	}
	_, _ = out.Write(append(data, '\n'))
}
