package clipboard

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Copy kinds, as the history names them.
const (
	kindText  = "text"
	kindHTML  = "html"
	kindRTF   = "rtf"
	kindImage = "image"
	kindFiles = "files"
)

// Shapes of a row's text: the title is one line of at most titleRunes characters, the subtitle one line of at most
// subtitleRunes.
const (
	titleRunes    = 72
	subtitleRunes = 90
	subtitleTime  = "Jan 2, 3:04 PM"
	infoTime      = "Jan 2 15:04"
)

// copyRow is one copy as a row: its title and subtitle, the copy action on Enter, and the actions on its menu. Images
// carry their thumbnail and their file, so the row draws the picture and drags out as an image file.
func copyRow(entry api.ClipboardEntry, score float64, canPaste bool, options Options) api.Item {
	row := api.Item{
		ID: "clipboard:" + entry.ID, Title: titleOf(entry), Subtitle: subtitleOf(entry), Icon: iconOf(entry.Type),
		Appearance: &api.Appearance{Foreground: api.ColorBlue}, Section: search.KindClipboard,
		LearnKey: "clipboard:" + entry.ID, Score: score, LastUsedMs: entry.CreatedAtMs,
		Detail: detailOf(entry, options),
	}
	if entry.Type == kindImage {
		attachPicture(&row, entry)
	}
	row.Primary = copyActionOf(entry)
	row.Actions = rowActions(entry, canPaste)
	return row
}

// attachPicture gives an image row its thumbnail, and its file for the drag out of the palette.
func attachPicture(row *api.Item, entry api.ClipboardEntry) {
	if entry.ThumbnailURL != "" {
		row.Image = &api.Image{Src: entry.ThumbnailURL, Alt: titleOf(entry), Fit: api.FitContain}
	}
	if entry.ImagePath != "" {
		file := api.FileRef{Path: entry.ImagePath, Name: titleOf(entry), Kind: "image"}
		row.File, row.DragImage = &file, &file
	}
}

// copyActionOf is the copy on Enter: plain text is copied as text, a picture by its file, and markup and file lists
// through the clipboard service, which keeps their formats.
func copyActionOf(entry api.ClipboardEntry) *api.Action {
	var action api.Action
	switch entry.Type {
	case kindImage:
		action = api.CopyImage(copyTitle(entry.Type), entry.ImagePath)
	case kindHTML:
		action = api.Run(refCopy, copyTitle(entry.Type), map[string]string{argID: entry.ID})
		action.DismissAfterRun = true
	case kindFiles:
		action = api.Run(refCopy, copyTitle(entry.Type), map[string]string{argID: entry.ID})
		action.DismissAfterRun = true
	default:
		action = api.CopyText(copyTitle(entry.Type), entry.Text, false)
		action.DismissAfterRun = true
	}
	return &action
}

// rowActions are the actions on a row's menu, after its copy: Preview, Paste Text where the copy holds text and the
// platform can paste, then the removals.
func rowActions(entry api.ClipboardEntry, canPaste bool) []api.Action {
	preview := api.Run(refPreview, "Preview", map[string]string{argID: entry.ID})
	preview.Shortcut = shortcutPreview
	actions := []api.Action{preview}
	if canPaste && isText(entry.Type) {
		paste := api.PasteText("Paste Text", entry.Text)
		paste.Shortcut = shortcutPasteText
		actions = append(actions, paste)
	}
	remove := api.Destructive(api.Run(refRemove, "Remove Item", map[string]string{argID: entry.ID}))
	remove.Shortcut = shortcutRemoveItem
	return append(actions, remove, removeLastAction())
}

// isText reports whether a copy holds text, with or without markup.
func isText(kind string) bool {
	return kind == kindText || kind == kindHTML || kind == kindRTF
}

// copyTitle names the copy action for what it copies.
func copyTitle(kind string) string {
	switch kind {
	case kindImage:
		return "Copy Image"
	case kindFiles:
		return "Copy Paths"
	}
	return "Copy Text"
}

// iconOf is the Lucide icon of a copy's kind.
func iconOf(kind string) string {
	switch kind {
	case kindImage:
		return "image"
	case kindFiles:
		return "file-text"
	}
	return "clipboard"
}

// titleOf is the first line of a copy, shortened to titleRunes characters. Pictures and file lists are named by kind.
func titleOf(entry api.ClipboardEntry) string {
	switch entry.Type {
	case kindImage:
		return "Clipboard image"
	case kindFiles:
		return fileTitle(entry.FilePath)
	}
	return truncate(oneLine(entry.Text), titleRunes)
}

// fileTitle is the last element of the first file of a file list.
func fileTitle(path string) string {
	if path == "" {
		return "Clipboard files"
	}
	if i := strings.LastIndexAny(path, `/\`); i >= 0 && i < len(path)-1 {
		return path[i+1:]
	}
	return path
}

// subtitleOf names the kind and the time of the copy.
func subtitleOf(entry api.ClipboardEntry) string {
	when := time.UnixMilli(entry.CreatedAtMs).Format(subtitleTime)
	switch entry.Type {
	case kindImage:
		return "Image copied " + when
	case kindFiles:
		return "Files copied " + when
	}
	return "Copied " + when
}

// detailOf is the side preview of a copy: its text or picture, the link card of a single web address, and the footer
// line with the age, the time and the size.
func detailOf(entry api.ClipboardEntry, options Options) *api.Detail {
	detail := &api.Detail{Title: titleOf(entry), Subtitle: subtitleOf(entry)}
	if entry.Type == kindImage {
		if entry.ThumbnailURL != "" {
			detail.Image = &api.Image{Src: entry.ThumbnailURL, Fit: api.FitContain}
		}
	} else {
		detail.Text = entry.Text
	}
	if link := singleWebURL(entry.Text); link != "" && options.LinkPreviews && isText(entry.Type) {
		detail.Link = &api.LinkCard{URL: link, Status: "loading"}
	}
	detail.Metadata = []api.Metadata{{Kind: api.MetadataText, Value: footerOf(entry)}}
	return detail
}

// footerOf is the footer of the side preview: when the copy was made, and for text its length.
func footerOf(entry api.ClipboardEntry) string {
	created := time.UnixMilli(entry.CreatedAtMs)
	info := fmt.Sprintf("%s · %s", ago(created), created.Format(infoTime))
	if !isText(entry.Type) {
		return info
	}
	return info + fmt.Sprintf(" · %d chars", len([]rune(entry.Text)))
}

// ago describes how long ago t was, in the words the rows use.
func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// singleWebURL returns text when it is exactly one http or https address, or "" otherwise.
func singleWebURL(text string) string {
	candidate := strings.TrimSpace(text)
	if candidate == "" || strings.ContainsAny(candidate, " \t\r\n") {
		return ""
	}
	parsed, err := url.Parse(candidate)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return ""
	}
	return candidate
}

// oneLine collapses s to one line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// truncate shortens s to limit runes, with an ellipsis when it is cut.
func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return s
}
