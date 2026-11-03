package app

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
	"github.com/pablopunk/nvm/native/internal/search"
)

// filePreview is what the preview pane shows for one version of a file. Exactly one of image and text is set, or
// neither, in which case note says why.
type filePreview struct {
	image *ui.Bitmap
	text  string
	note  string
}

// detail returns the preview pane of a file row. The pane draws from the preview cache: a file not yet read shows a
// placeholder and is read on a worker, and the redraw that follows shows it.
func (s *fileService) detail(r files.Result) func(c *ui.Context, h search.Host) {
	return func(c *ui.Context, _ search.Host) {
		s.drawPreview(c, r)
	}
}

// drawPreview draws the preview of r and its metadata. It runs on the UI thread and reads only the cache.
func (s *fileService) drawPreview(c *ui.Context, r files.Result) {
	t := design.Tokens(c)
	pv, ready := s.previews.Get(previewKey(r), func() *filePreview { return s.loadPreview(r) })
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		switch {
		case !ready:
			ui.Text(c, "Loading preview…").FontSize(12).TextColor(t.Tertiary)
		case pv.image != nil:
			ui.Image(c, pv.image).Fill().Fit(ui.Contain)
		case pv.text != "":
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Text(c, pv.text).FontSize(13).TextColor(t.Primary)
			})
		case pv.note != "":
			ui.Text(c, pv.note).FontSize(12).TextColor(t.Secondary)
		}
		if ready && pv.note != "" && pv.text != "" {
			ui.Text(c, pv.note).FontSize(11).TextColor(t.Tertiary)
		}
	})
	ui.Divider(c)
	for _, line := range metadataRows(r) {
		ui.Text(c, line).FontSize(11).TextColor(t.Tertiary)
	}
}

// loadPreview reads the preview of r. It runs on a worker. Images are decoded to a bounded picture, text is read up to
// the preview limit, and videos get no picture: their preview is the metadata beside it.
func (s *fileService) loadPreview(r files.Result) *filePreview {
	switch r.Kind {
	case files.KindImage:
		return s.imagePreview(r)
	case files.KindVideo:
		return &filePreview{note: "No picture preview for this video."}
	default:
		return s.textPreview(r)
	}
}

// imagePreview renders the picture of an image at preview size. Files over the size cap, and files that do not decode,
// say that no preview is available.
func (s *fileService) imagePreview(r files.Result) *filePreview {
	const unavailable = "Preview unavailable"
	if r.Size > thumbs.MaxSourceBytes {
		return &filePreview{note: unavailable}
	}
	data, err := thumbs.Render(s.fsys, r.Path, previewEdge)
	if err != nil {
		return &filePreview{note: unavailable}
	}
	bm := thumbnailBitmap(data)
	if bm == nil {
		return &filePreview{note: unavailable}
	}
	return &filePreview{image: bm}
}

// textPreview reads the beginning of a file as text. Binary files and unreadable files get the 1.x wording.
func (s *fileService) textPreview(r files.Result) *filePreview {
	p, err := files.TextPreviewDetail(s.fsys, r.Path)
	if err != nil {
		return &filePreview{note: "Could not read this file: " + err.Error()}
	}
	if p.Binary {
		return &filePreview{note: fmt.Sprintf("Binary file (%s) — no text preview.", kilobyteLabel(p.SizeBytes))}
	}
	note := ""
	if p.Truncated {
		note = fmt.Sprintf("Showing the first %s of %s.", kilobyteLabel(int64(len(p.Text))), kilobyteLabel(p.SizeBytes))
	}
	return &filePreview{text: p.Text, note: note}
}

// previewKey identifies one version of a file, so an edited file is previewed again.
func previewKey(r files.Result) string {
	return r.Path + "|" + strconv.FormatInt(r.Modified.UnixMilli(), 10) + "|" + strconv.FormatInt(r.Size, 10)
}

// metadataRows describes a file in the words the preview pane uses: kind, size, modification time and path.
func metadataRows(r files.Result) []string {
	kind := "File"
	switch r.Kind {
	case files.KindImage:
		kind = "Image"
	case files.KindVideo:
		kind = "Video"
	}
	rows := []string{"Kind: " + kind, "Size: " + byteLabel(r.Size)}
	if !r.Modified.IsZero() {
		rows = append(rows, "Modified: "+timeLabel(r.Modified))
	}
	return append(rows, "Path: "+r.DisplayPath)
}

// kilobyteLabel formats a byte count as whole kilobytes, rounding halves up as the palette always has.
func kilobyteLabel(n int64) string {
	return fmt.Sprintf("%d KB", int64(math.Floor(float64(n)/1024+0.5)))
}

// byteLabel formats a size for a person: bytes, kilobytes or megabytes.
func byteLabel(n int64) string {
	switch {
	case n < 1024:
		return strconv.FormatInt(n, 10) + " B"
	case n < 1<<20:
		return kilobyteLabel(n)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
}

// timeLabel formats a modification time for the metadata rows.
func timeLabel(t time.Time) string {
	return t.Format("Jan 2, 2006 15:04")
}
