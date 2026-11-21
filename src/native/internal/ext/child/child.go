// Package child is the extension process. The host starts the Nevermind binary again with Flag, the extension
// source path, a memory limit and the extension's identity as JSON. This package interprets the source with
// Yaegi and answers line-delimited requests on stdin with responses on stdout (see package protocol). Extension
// code can crash, hang or exhaust memory in this process only; the host kills it at the deadline.
//
// Extension source imports the api package (through the generated symbol table in api_symbols.go) and the
// standard library. Its entry points have the signatures of the api interfaces (see docs/extension-api.md).
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
	"github.com/pablopunk/nvm/native/internal/platform/real"
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
	return serveIn(modeOf(args), args[2], limit, identity, os.Stdin, protocolOut), true
}

func argOr(args []string, index int) string {
	if index < len(args) {
		return args[index]
	}
	return "{}"
}

// modeOf says whether the child runs an extension or one AI conversation program.
func modeOf(args []string) mode {
	if len(args) > 5 && args[5] == protocol.ProgramArg {
		return modeProgram
	}
	return modeExtension
}

func parseIdentity(text string) (api.Identity, error) {
	var identity api.Identity
	err := json.Unmarshal([]byte(text), &identity)
	return identity, err
}

// serve applies the memory limit, loads the extension, announces it, and answers requests until stdin ends.
func serve(sourcePath string, memoryLimit uint64, identity api.Identity, in io.Reader, out io.Writer) int {
	return serveIn(modeExtension, sourcePath, memoryLimit, identity, in, out)
}

// serveIn is serve for either kind of child.
func serveIn(m mode, sourcePath string, memoryLimit uint64, identity api.Identity, in io.Reader, out io.Writer) int {
	if memoryLimit > 0 {
		if err := applyMemoryLimit(memoryLimit); err != nil {
			fmt.Fprintf(os.Stderr, "extension child: memory limit not applied: %v\n", err)
		}
	}
	link := newHostLink(out)
	ext, err := loadIn(m, sourcePath, identity, real.NewPorts().Clock, link)
	if err != nil {
		failure := api.FailureOf(err)
		failure.Message = truncate(failure.Message)
		writeLineLocked(out, protocol.Ready{OK: false, Failure: &failure})
		return 1
	}
	writeLineLocked(out, protocol.Ready{OK: true, Methods: ext.declaredMethods()})
	return answerRequests(ext, readInbound(in, link), link)
}

// readInbound reads host lines until stdin ends. Service replies go straight to the call waiting for them, so
// a request that is being answered can finish its service calls; every other line is a request, queued for the
// answering loop. The host runs one request at a time, so the queue holds at most one line.
func readInbound(in io.Reader, link *hostLink) <-chan protocol.Request {
	requests := make(chan protocol.Request, 1)
	go func() {
		defer close(requests)
		defer link.close()
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 0, 64<<10), protocol.MaxLineBytes)
		for scanner.Scan() {
			routeInboundLine(scanner.Bytes(), link, requests)
		}
	}()
	return requests
}

// routeInboundLine hands one host line to the call that awaits it, or queues it as a request.
func routeInboundLine(line []byte, link *hostLink, requests chan<- protocol.Request) {
	if protocol.IsServiceReply(line) {
		var reply protocol.ServiceReply
		if err := json.Unmarshal(line, &reply); err == nil {
			link.deliver(reply)
		}
		return
	}
	var req protocol.Request
	if err := json.Unmarshal(line, &req); err != nil {
		link.send(protocol.Response{Failure: &api.Failure{Code: api.CodeInvalid, Message: "request is not valid JSON"}})
		return
	}
	requests <- req
}

// answerRequests runs the queued requests in order and writes one response per request.
func answerRequests(ext *extension, requests <-chan protocol.Request, link *hostLink) int {
	for req := range requests {
		link.sendResponse(ext.answer(req), req.Method)
	}
	return 0
}

// writeLineLocked marshals v and writes it as one line. A write error ends the child: the host has gone.
func writeLineLocked(out io.Writer, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		data = []byte(`{"failure":{"code":"internal","message":"response cannot be encoded"}}`)
	}
	_, _ = out.Write(append(data, '\n'))
}
