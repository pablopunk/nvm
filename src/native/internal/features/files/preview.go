package files

import (
	"bytes"
	"fmt"
	"math"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// TextPreviewLimit bounds how much of a file a text preview reads and keeps.
const TextPreviewLimit = 64 * 1024

// binarySniffBytes is how much of a file is searched for a NUL byte to decide it is binary.
const binarySniffBytes = 8 * 1024

// unicodeReplacement is what invalid UTF-8 decodes to, and what a cut-off trailing character decodes to.
const unicodeReplacement = "�"

// Preview is what a text preview read found.
type Preview struct {
	Text      string
	Binary    bool
	Truncated bool
	SizeBytes int64
	// Err is set when the file could not be read; the other fields are then zero.
	Err error
}

// TextPreview reads the beginning of a file for display. It returns the text, whether the file looks binary, whether
// the text stops before the end of the file, and the read error if any. Binary files have no text.
func TextPreview(fsys platform.FS, path string) (text string, binary bool, truncated bool, err error) {
	preview, err := TextPreviewDetail(fsys, path)
	return preview.Text, preview.Binary, preview.Truncated, err
}

// TextPreviewDetail is TextPreview with the file size and the read error in one value.
func TextPreviewDetail(fsys platform.FS, path string) (Preview, error) {
	return textPreview(fsys, path, TextPreviewLimit)
}

// textPreview reads at most limit bytes of path and decodes them the way the text preview does.
func textPreview(fsys platform.FS, path string, limit int64) (Preview, error) {
	data, size, err := fsys.ReadFilePrefix(path, limit)
	if err != nil {
		return Preview{Err: err}, err
	}
	if bytes.IndexByte(data[:min(len(data), binarySniffBytes)], 0) >= 0 {
		return Preview{Binary: true, SizeBytes: size}, nil
	}
	truncated := size > int64(len(data))
	text := strings.ToValidUTF8(string(data), unicodeReplacement)
	if truncated && strings.HasSuffix(text, unicodeReplacement) {
		text = strings.TrimSuffix(text, unicodeReplacement)
	}
	return Preview{Text: text, Truncated: truncated, SizeBytes: size}, nil
}

// MarkdownPreview formats a preview as Markdown for a palette view: the text in a fence longer than any backtick run it
// contains, a note when the text was cut, or a one-line note for binary and unreadable files.
func MarkdownPreview(filePath string, p Preview) string {
	switch {
	case p.Err != nil:
		return fmt.Sprintf("_Could not read this file: %s_", p.Err.Error())
	case p.Binary:
		return fmt.Sprintf("_Binary file (%s) — no text preview._", kilobytes(p.SizeBytes))
	}
	fence := fenceFor(p.Text)
	block := fence + ExtensionOf(filePath) + "\n" + p.Text + "\n" + fence
	if !p.Truncated {
		return block
	}
	return fmt.Sprintf("%s\n\n_Showing the first %s of %s._", block, kilobytes(int64(len(p.Text))), kilobytes(p.SizeBytes))
}

// fenceFor returns a backtick fence one longer than the longest backtick run in text, and never shorter than three.
func fenceFor(text string) string {
	longest := 2
	run := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '`' {
			run++
			longest = max(longest, run)
			continue
		}
		run = 0
	}
	return strings.Repeat("`", longest+1)
}

// kilobytes formats a byte count as whole kilobytes, rounding halves up as the palette always has.
func kilobytes(n int64) string {
	return fmt.Sprintf("%d KB", int64(math.Floor(float64(n)/1024+0.5)))
}
