package real

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// wlPasteBinary is the Wayland clipboard tool from wl-clipboard. Wayland clients cannot read the clipboard while
// they are not focused, but wl-paste does it through the data-control protocol, which is why clipboard history
// reads copies through it and not through the toolkit.
const wlPasteBinary = "wl-paste"

// waylandWatchTools are the programs the Wayland clipboard feed needs: wl-paste watches, and the shell and base64
// frame each copy as one line on the watcher's output.
var waylandWatchTools = []string{wlPasteBinary, "sh", "base64"}

// maxFeedCopyBytes bounds one copy the feed accepts. A larger copy is skipped, not truncated.
const maxFeedCopyBytes = 64 << 20

// Restart policy of the watch process: it is restarted after it exits, and given up on after this many starts in
// a row that produced nothing (a compositor without the data-control protocol makes wl-paste exit at once).
const (
	maxEmptyWatchStarts = 3
	watchRestartDelay   = time.Second
)

// waylandWatchScript runs once per clipboard change inside `wl-paste --watch`, with the copy on its standard
// input. wl-paste waits for it to exit, so runs never interleave. It writes one line, "<state> <mime-type> <base64>".
// The state is data, sensitive (a password manager asked not to be recorded; the content is discarded unread) or
// nil (the clipboard was emptied). wl-clipboard releases differ in what they export: a missing state means data,
// and a missing type is written as "-" so the host recognises the content from its bytes.
const waylandWatchScript = `case "${CLIPBOARD_STATE:-data}" in
sensitive) cat >/dev/null; printf 'sensitive - \n';;
nil|clear) cat >/dev/null; printf 'nil - \n';;
*) printf 'data %s ' "${CLIPBOARD_TYPE:--}"; base64 -w0; printf '\n';;
esac`

// streamStarter starts a long-running command and returns its standard output. Closing the stream stops the
// command. Tests pass fakes; the production one is startProcessStream.
type streamStarter func(ctx context.Context, binary string, args ...string) (io.ReadCloser, error)

// waylandFeed implements platform.ClipboardFeed with `wl-paste --watch` on a Wayland session. On any other
// session it reports that no feed is needed.
type waylandFeed struct {
	wayland      bool
	hasTool      func(string) bool
	start        streamStarter
	restartDelay time.Duration
}

var _ platform.ClipboardFeed = (*waylandFeed)(nil)

// newWaylandFeed returns the feed for the given OS and session.
func newWaylandFeed(goos string, getenv func(string) string, hasTool func(string) bool, start streamStarter) *waylandFeed {
	return &waylandFeed{
		wayland:      isWaylandSession(goos, getenv),
		hasTool:      hasTool,
		start:        start,
		restartDelay: watchRestartDelay,
	}
}

// WatchCopies starts the watch process and delivers each copy until ctx is done.
func (f *waylandFeed) WatchCopies(ctx context.Context) (<-chan platform.ClipboardCopy, error) {
	if !f.wayland {
		return nil, nil
	}
	for _, tool := range waylandWatchTools {
		if !f.hasTool(tool) {
			return nil, fmt.Errorf("%w: %s is not installed (wl-clipboard provides wl-paste)", platform.ErrUnsupported, tool)
		}
	}
	stream, err := f.openWatch(ctx)
	if err != nil {
		return nil, err
	}
	out := make(chan platform.ClipboardCopy, 32)
	go f.run(ctx, stream, out)
	return out, nil
}

// openWatch starts one wl-paste watch process.
func (f *waylandFeed) openWatch(ctx context.Context) (io.ReadCloser, error) {
	return f.start(ctx, wlPasteBinary, "--watch", "sh", "-c", waylandWatchScript)
}

// run reads copies from the current stream and restarts the watch process when it exits, until ctx is done or
// the process keeps exiting without a single line. It closes out on return.
func (f *waylandFeed) run(ctx context.Context, stream io.ReadCloser, out chan<- platform.ClipboardCopy) {
	defer close(out)
	first := true
	emptyStarts := 0
	for {
		lines := f.deliver(ctx, stream, out, first)
		_ = stream.Close()
		first = false
		if ctx.Err() != nil {
			return
		}
		if lines == 0 {
			emptyStarts++
		} else {
			emptyStarts = 0
		}
		if emptyStarts >= maxEmptyWatchStarts {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(f.restartDelay):
		}
		next, err := f.openWatch(ctx)
		if err != nil {
			return
		}
		stream = next
	}
}

// deliver reads the lines of one process and sends the copies they hold. It returns how many lines it read. The
// first line of the first process describes what was on the clipboard before the feed started.
func (f *waylandFeed) deliver(ctx context.Context, stream io.Reader, out chan<- platform.ClipboardCopy, first bool) int {
	reader := bufio.NewReaderSize(stream, 64<<10)
	lines := 0
	for {
		line, err := readFeedLine(reader, maxFeedCopyBytes)
		if line != nil || errors.Is(err, errFeedLineTooLong) {
			lines++
			copied, ok := parseWaylandCopy(line)
			copied.Initial = first && lines == 1
			if ok && !sendCopy(ctx, out, copied) {
				return lines
			}
		}
		if err != nil && !errors.Is(err, errFeedLineTooLong) {
			return lines
		}
	}
}

// sendCopy delivers one copy, or reports false when ctx ended first.
func sendCopy(ctx context.Context, out chan<- platform.ClipboardCopy, copied platform.ClipboardCopy) bool {
	select {
	case out <- copied:
		return true
	case <-ctx.Done():
		return false
	}
}

// errFeedLineTooLong reports a line that exceeded the copy bound. The reader has skipped to the end of it.
var errFeedLineTooLong = errors.New("clipboard feed: copy too large")

// readFeedLine reads one line without its newline. A line whose decoded content would exceed limit is skipped to
// its end and reported as errFeedLineTooLong with a nil line. At end of input a partial last line is dropped.
func readFeedLine(reader *bufio.Reader, limit int) ([]byte, error) {
	var line []byte
	tooLong := false
	maxEncoded := limit/3*4 + 64
	for {
		chunk, err := reader.ReadSlice('\n')
		if !tooLong {
			line = append(line, chunk...)
			tooLong = len(line) > maxEncoded
		}
		switch {
		case err == nil:
			if tooLong {
				return nil, errFeedLineTooLong
			}
			return bytes.TrimSuffix(line, []byte("\n")), nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		default:
			return nil, err
		}
	}
}

// parseWaylandCopy turns one framed line into a copy. It reports false for events that are not a copy to record:
// an emptied clipboard, an unreadable line, or content that is neither text nor PNG.
func parseWaylandCopy(line []byte) (platform.ClipboardCopy, bool) {
	fields := strings.SplitN(string(line), " ", 3)
	if len(fields) != 3 {
		return platform.ClipboardCopy{}, false
	}
	switch fields[0] {
	case "sensitive":
		return platform.ClipboardCopy{Concealed: true}, true
	case "data":
	default:
		return platform.ClipboardCopy{}, false
	}
	data, err := base64.StdEncoding.DecodeString(fields[2])
	if err != nil || len(data) == 0 {
		return platform.ClipboardCopy{}, false
	}
	kind := feedContentKind(fields[1])
	if fields[1] == "-" || fields[1] == "" {
		kind = sniffedContentKind(data)
	}
	switch kind {
	case feedText:
		return platform.ClipboardCopy{Text: string(data)}, true
	case feedPNG:
		return platform.ClipboardCopy{PNG: data}, true
	case feedURIList:
		if paths := localFilePaths(string(data)); len(paths) > 0 {
			return platform.ClipboardCopy{Files: paths}, true
		}
		return platform.ClipboardCopy{Text: string(data)}, true
	}
	return platform.ClipboardCopy{}, false
}

type feedContent int

const (
	feedOther feedContent = iota
	feedText
	feedPNG
	feedURIList
)

// feedContentKind classifies the MIME type wl-paste picked for a copy. The legacy X11 text target names appear
// when the owner is an XWayland client.
func feedContentKind(mime string) feedContent {
	base, _, _ := strings.Cut(strings.ToLower(mime), ";")
	switch {
	case base == "image/png":
		return feedPNG
	case base == "text/uri-list":
		return feedURIList
	case strings.HasPrefix(base, "text/"), base == "utf8_string", base == "string", base == "text":
		return feedText
	}
	return feedOther
}

// sniffedContentKind recognises a copy whose MIME type wl-paste did not report: PNG by its signature, a file-manager
// copy by its file URIs, and any other valid UTF-8 as text.
func sniffedContentKind(data []byte) feedContent {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return feedPNG
	case !utf8.Valid(data):
		return feedOther
	case len(localFilePaths(string(data))) > 0:
		return feedURIList
	}
	return feedText
}
