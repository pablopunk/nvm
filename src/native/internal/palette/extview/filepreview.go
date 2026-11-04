package extview

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

const (
	// previewBytes bounds what a file preview reads: the leading 64 KB, as the Electron host does.
	previewBytes = 64 << 10
	// sniffBytes is how much of the head a NUL byte is looked for in, to tell a binary file from text.
	sniffBytes = 8 << 10
)

// filePreviewPage previews a file the host reads, never the extension. The read is bounded to the leading bytes,
// so opening the page does not wait on a large file.
func (s *Session) filePreviewPage() *palette.View {
	source := s.fileMarkdown(s.view.File.File)
	return &palette.View{
		Title: s.view.Title, HideInput: true,
		Body: func(c *ui.Context, _ search.Host) {
			ui.Scroll(c).Grow(1).Children(func() { drawMarkdown(c, source, s.markdown) })
		},
	}
}

// fileMarkdown is the preview of a file as Markdown: a fenced block of its leading text, a note for binary content, or
// the reason it cannot be read.
func (s *Session) fileMarkdown(file api.FileRef) string {
	if s.deps.Files == nil {
		return "_File previews are not available in this build._"
	}
	data, size, err := s.deps.Files.ReadFilePrefix(file.Path, previewBytes)
	if err != nil {
		return "_Could not read this file: " + err.Error() + "_"
	}
	head := data[:min(len(data), sniffBytes)]
	if bytes.IndexByte(head, 0) >= 0 {
		return fmt.Sprintf("_Binary file (%d KB) — no text preview._", kilobytes(size))
	}
	return textPreviewMarkdown(string(data), file.Extension, size)
}

// textPreviewMarkdown fences the text in a block longer than any backtick run it holds, and notes when the file is longer
// than what was read.
func textPreviewMarkdown(text, extension string, size int64) string {
	fence := strings.Repeat("`", max(3, longestBacktickRun(text)+1))
	body := fence + strings.TrimPrefix(extension, ".") + "\n" + text + "\n" + fence
	if size > int64(len(text)) {
		body += fmt.Sprintf("\n\n_Showing the first %d KB of %d KB._", kilobytes(int64(len(text))), kilobytes(size))
	}
	return body
}

func longestBacktickRun(text string) int {
	longest, run := 0, 0
	for i := 0; i < len(text); i++ {
		if text[i] == '`' {
			run++
			longest = max(longest, run)
			continue
		}
		run = 0
	}
	return longest
}

func kilobytes(bytes int64) int64 { return (bytes + 512) / 1024 }
