package backend

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// maxLineBytes bounds one SSE line. A longer line fails the stream instead of growing memory without limit.
const maxLineBytes = 8 << 20

// lineReaderBuffer is the bufio size; ReadSlice returns a line in pieces of at most this many bytes.
const lineReaderBuffer = 32 << 10

// errLineTooLong reports a line over maxLineBytes.
var errLineTooLong = errors.New("sse line exceeds the size limit")

// sseFrame is one dispatched server-sent event: its event name (empty when unnamed) and its data, with
// multiple data lines joined by newlines as the SSE format specifies.
type sseFrame struct {
	event string
	data  string
}

// lineReader yields complete lines from a body. A line ends with LF or CRLF; a bare CR is not a line
// ending, which the OpenAI-compatible streams this client reads never use. A partial line at the end of the
// body is not returned, because the body ended in the middle of it.
type lineReader struct {
	reader *bufio.Reader
}

// newLineReader wraps body. Reads may split lines at any byte; the reader joins the pieces.
func newLineReader(body io.Reader) *lineReader {
	return &lineReader{reader: bufio.NewReaderSize(body, lineReaderBuffer)}
}

// next returns the next line without its line ending. It returns io.EOF when the body ends, including in the
// middle of a line.
func (l *lineReader) next() (string, error) {
	var line []byte
	for {
		piece, err := l.reader.ReadSlice('\n')
		line = append(line, piece...)
		if len(line) > maxLineBytes {
			return "", errLineTooLong
		}
		switch {
		case err == nil:
			return trimLineEnding(line), nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		default:
			return "", err
		}
	}
}

// trimLineEnding removes a trailing LF and then a trailing CR.
func trimLineEnding(line []byte) string {
	text := strings.TrimSuffix(string(line), "\n")
	return strings.TrimSuffix(text, "\r")
}

// frameReader groups lines into dispatched events, following the server-sent events rules: a blank line ends
// an event, lines starting with a colon are comments, and an event without data is not dispatched.
type frameReader struct {
	lines *lineReader
	event string
	data  []string
}

// newFrameReader reads events from body.
func newFrameReader(body io.Reader) *frameReader {
	return &frameReader{lines: newLineReader(body)}
}

// next returns the next dispatched event. At the end of the body it returns io.EOF and discards any event
// that was not finished by a blank line, as the format requires.
func (f *frameReader) next() (sseFrame, error) {
	for {
		line, err := f.lines.next()
		if err != nil {
			f.reset()
			return sseFrame{}, err
		}
		if line == "" {
			if frame, ok := f.dispatch(); ok {
				return frame, nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		f.absorb(line)
	}
}

// absorb applies one field line: data appends a line of data, event names the event, and other fields are
// ignored because the chat stream does not use them.
func (f *frameReader) absorb(line string) {
	field, value, _ := strings.Cut(line, ":")
	value = strings.TrimPrefix(value, " ")
	switch field {
	case "data":
		f.data = append(f.data, value)
	case "event":
		f.event = value
	}
}

// dispatch returns the pending event when it has data, and clears the buffer either way.
func (f *frameReader) dispatch() (sseFrame, bool) {
	if len(f.data) == 0 {
		f.reset()
		return sseFrame{}, false
	}
	frame := sseFrame{event: f.event, data: strings.Join(f.data, "\n")}
	f.reset()
	return frame, true
}

// reset clears the pending event.
func (f *frameReader) reset() {
	f.event = ""
	f.data = nil
}
