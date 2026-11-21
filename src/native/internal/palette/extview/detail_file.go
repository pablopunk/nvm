package extview

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// filePreviewText is the leading text of a file for a detail pane: the text, or the note that replaces it.
type filePreviewText struct {
	text string
	note string
}

// drawFileDetail draws a file's preview above its metadata, as the Files view's side pane does. It runs on the UI thread
// and reads only what the session has already read; the first draw of a file reads its leading bytes.
func (s *Session) drawFileDetail(c *ui.Context, file api.FileRef) {
	t := design.Tokens(c)
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		s.drawFilePreview(c, file, t)
	})
	ui.Divider(c)
	for _, line := range fileMetadataLines(file) {
		ui.Text(c, line).FontSize(11).TextColor(t.Tertiary)
	}
}

func (s *Session) drawFilePreview(c *ui.Context, file api.FileRef, t *design.TokenSet) {
	switch file.Kind {
	case "image":
		s.requestImage(file.Path)
		if bm := s.images[file.Path]; bm != nil {
			ui.Image(c, bm).Fill().Fit(ui.Contain)
			return
		}
		ui.Text(c, "Loading preview…").FontSize(12).TextColor(t.Tertiary)
	case "video":
		ui.Text(c, "No picture preview for this video.").FontSize(12).TextColor(t.Secondary)
	default:
		s.drawLeadingText(c, s.leadingText(file), t)
	}
}

func (s *Session) drawLeadingText(c *ui.Context, preview filePreviewText, t *design.TokenSet) {
	if preview.text != "" {
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Text(c, preview.text).FontSize(13).TextColor(t.Primary)
		})
	}
	if preview.note != "" {
		ui.Text(c, preview.note).FontSize(12).TextColor(t.Secondary)
	}
}

// leadingText is the preview text of a file, read once per version of the file and kept for the session's draws.
func (s *Session) leadingText(file api.FileRef) filePreviewText {
	key := file.Path + "|" + strconv.FormatInt(file.MtimeMs, 10) + "|" + strconv.FormatInt(file.Size, 10)
	if preview, ok := s.filePreviews[key]; ok {
		return preview
	}
	if s.filePreviews == nil {
		s.filePreviews = map[string]filePreviewText{}
	}
	preview := s.readLeadingText(file.Path)
	s.filePreviews[key] = preview
	return preview
}

// readLeadingText reads the leading bytes of a file as text. A binary file, or one that cannot be read, gets a note
// instead, in the words the Files view uses.
func (s *Session) readLeadingText(path string) filePreviewText {
	if s.deps.Files == nil {
		return filePreviewText{}
	}
	data, size, err := s.deps.Files.ReadFilePrefix(path, previewBytes)
	if err != nil {
		return filePreviewText{note: "Could not read this file: " + err.Error()}
	}
	if bytes.IndexByte(data[:min(len(data), sniffBytes)], 0) >= 0 {
		return filePreviewText{note: fmt.Sprintf("Binary file (%d KB) — no text preview.", kilobytes(size))}
	}
	note := ""
	if size > int64(len(data)) {
		note = fmt.Sprintf("Showing the first %d KB of %d KB.", kilobytes(int64(len(data))), kilobytes(size))
	}
	return filePreviewText{text: string(data), note: note}
}

// fileMetadataLines describes a file in the words the Files view's pane uses: kind, size, modified time and path.
func fileMetadataLines(file api.FileRef) []string {
	kind := "File"
	switch file.Kind {
	case "image":
		kind = "Image"
	case "video":
		kind = "Video"
	}
	lines := []string{"Kind: " + kind, "Size: " + byteLabel(file.Size)}
	if file.MtimeMs != 0 {
		lines = append(lines, "Modified: "+time.UnixMilli(file.MtimeMs).Format("Jan 2, 2006 15:04"))
	}
	return append(lines, "Path: "+file.DisplayPath)
}

// byteLabel formats a size for a person: bytes, kilobytes or megabytes.
func byteLabel(n int64) string {
	switch {
	case n < 1024:
		return strconv.FormatInt(n, 10) + " B"
	case n < 1<<20:
		return fmt.Sprintf("%d KB", kilobytes(n))
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
}
